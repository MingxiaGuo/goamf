package amf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Reference bit.
const REFERENCE_BIT = 0x01
const ExtObj = "nilGoNullNilExtObj"

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type AvmObject struct {
	Class         *AvmClass
	StaticFields  map[string]interface{}
	DynamicFields map[string]interface{}
}

type AvmClass struct {
	Name           string
	Externalizable bool
	Dynamic        bool
	Properties     []string
}

// An "Array" in AVM land is actually stored as a combination of an array and
// a dictionary.
type AvmArray struct {
	elements []interface{}
	fields   map[string]interface{}
}

// * Public functions *

// Read an AMF3 value from the stream.
func ReadValueAmf3(stream Reader) (interface{}, error) {
	cxt := &Decoder{}
	cxt.AmfVersion = 3
	cxt.stream = stream
	result := cxt.ReadValueAmf3()
	return result, cxt.decodeError
}

func WriteValueAmf3(stream Writer, value interface{}) error {
	cxt := &Encoder{}
	cxt.stream = stream
	return cxt.WriteValueAmf3(value)
}

// Type markers
const (
	amf0_numberType        = 0
	amf0_booleanType       = 1
	amf0_stringType        = 2
	amf0_objectType        = 3
	amf0_movieClipType     = 4
	amf0_nullType          = 5
	amf0_undefinedType     = 6
	amf0_referenceType     = 7
	amf0_ecmaArrayType     = 8
	amf0_objectEndType     = 9
	amf0_strictArrayType   = 10
	amf0_dateType          = 11
	amf0_longStringType    = 12
	amf0_unsupporedType    = 13
	amf0_recordsetType     = 14
	amf0_xmlObjectType     = 15
	amf0_typedObjectType   = 16
	amf0_avmPlusObjectType = 17

	amf3_undefinedType  = 0
	amf3_nullType       = 1
	amf3_falseType      = 2
	amf3_trueType       = 3
	amf3_integerType    = 4
	amf3_doubleType     = 5
	amf3_stringType     = 6
	amf3_xmlType        = 7
	amf3_dateType       = 8
	amf3_arrayType      = 9
	amf3_objectType     = 10
	amf3_avmPlusXmlType = 11
	amf3_byteArrayType  = 12
)

type Decoder struct {
	stream Reader

	AmfVersion uint16

	IsAMF3 bool

	// AMF3 messages can include references to previously-unpacked objects. These
	// tables hang on to objects for later use.
	stringTable []string
	classTable  []*AvmClass
	objectTable []interface{}

	decodeError error

	// When unpacking objects, we'll look in this map for the type name. If found,
	// we'll unpack the value into an instance of the associated type.
	typeMap map[string]reflect.Type
}

func NewDecoder(stream Reader, amfVersion uint16) *Decoder {
	decoder := &Decoder{}
	decoder.stream = stream
	decoder.AmfVersion = amfVersion
	decoder.typeMap = make(map[string]reflect.Type)
	return decoder
}

func (cxt *Decoder) useAmf3() bool {
	return cxt.AmfVersion == 3
}
func (cxt *Decoder) saveError(err error) {
	if err == nil {
		return
	}
	if cxt.decodeError != nil {
		fmt.Println("warning: duplicate errors on Decoder")
	} else {
		cxt.decodeError = err
	}
}
func (cxt *Decoder) errored() bool {
	return cxt.decodeError != nil
}
func (cxt *Decoder) storeObjectInTable(obj interface{}) {
	cxt.objectTable = append(cxt.objectTable, obj)
}
func (cxt *Decoder) RegisterType(flexName string, instance interface{}) {
	cxt.typeMap[flexName] = reflect.TypeOf(instance)
}

// Helper functions.
func (cxt *Decoder) ReadByte() uint8 {
	buf := make([]byte, 1)
	_, err := cxt.stream.Read(buf)
	cxt.saveError(err)
	return buf[0]
}
func (cxt *Decoder) ReadUint8() uint8 {
	var value uint8
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func (cxt *Decoder) ReadUint16() uint16 {
	var value uint16
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}

func (cxt *Decoder) Clear() {
	cxt.IsAMF3 = false
	cxt.stringTable = []string{}
	cxt.classTable = []*AvmClass{}
	cxt.objectTable = []interface{}{}
	cxt.decodeError = nil
	cxt.typeMap = make(map[string]reflect.Type)
	return
}

func (cxt *Decoder) ReadUint32() uint32 {
	var value uint32
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func (cxt *Decoder) ReadFloat64() float64 {
	var value float64
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func (cxt *Encoder) WriteFloat64(value float64) error {
	return binary.Write(cxt.stream, binary.BigEndian, &value)
}
func (cxt *Decoder) ReadString() (int, string) {
	length := int(cxt.ReadUint16())
	if cxt.errored() {
		return 0, ""
	}
	return length, cxt.ReadStringKnownLength(length)
}

func (cxt *Decoder) ReadStringKnownLength(length int) string {
	data := make([]byte, length)
	n, err := cxt.stream.Read(data)
	if n < length {
		cxt.saveError(errors.New(fmt.Sprintf(
			"Not enough bytes in ReadStringKnownLength (expected %d, found %d)", length, n)))
		return ""
	}
	cxt.saveError(err)
	return string(data)
}

type Encoder struct {
	stream Writer
}

func NewEncoder(stream Writer) *Encoder {
	return &Encoder{stream}
}
func (cxt *Encoder) WriteUint8(value uint8) error {
	return binary.Write(cxt.stream, binary.BigEndian, &value)
}
func (cxt *Encoder) WriteUint16(value uint16) error {
	return binary.Write(cxt.stream, binary.BigEndian, &value)
}
func (cxt *Encoder) WriteUint32(value uint32) error {
	return binary.Write(cxt.stream, binary.BigEndian, &value)
}

func (cxt *Encoder) WriteString(str string) error {
	binary.Write(cxt.stream, binary.BigEndian, uint16(len(str)))
	_, err := cxt.stream.Write([]byte(str))
	return err
}
func (cxt *Encoder) writeByte(b uint8) error {
	return binary.Write(cxt.stream, binary.BigEndian, b)
}
func (cxt *Encoder) WriteBool(b bool) {
	val := 0x0
	if b {
		val = 0xff
	}
	binary.Write(cxt.stream, binary.BigEndian, uint8(val))
}

// Read a 29-bit compact encoded integer (as defined in AVM3)
func (cxt *Decoder) ReadUint29() uint32 {
	var result uint32 = 0
	for i := 0; i < 4; i++ {
		b := cxt.ReadByte()

		if cxt.errored() {
			return 0
		}

		if i == 3 {
			// Last byte does not use the special 0x80 bit.
			result = (result << 8) + uint32(b)
		} else {
			result = (result << 7) + (uint32(b) & 0x7f)
		}

		if (b & 0x80) == 0 {
			break
		}
	}
	return result
}

func (cxt *Encoder) WriteUint29(value uint32) error {

	// Make sure the value is only 29 bits.
	remainder := value & 0x1fffffff
	if remainder != value {
		fmt.Println("warning: WriteUint29 received a value that does not fit in 29 bits")
	}

	if remainder > 0x1fffff {
		cxt.writeByte(uint8(remainder>>22)&0x7f + 0x80)
		cxt.writeByte(uint8(remainder>>15)&0x7f + 0x80)
		cxt.writeByte(uint8(remainder>>8)&0x7f + 0x80)
		cxt.writeByte(uint8(remainder>>0) & 0xff)
	} else if remainder > 0x3fff {
		cxt.writeByte(uint8(remainder>>14)&0x7f + 0x80)
		cxt.writeByte(uint8(remainder>>7)&0x7f + 0x80)
		cxt.writeByte(uint8(remainder>>0) & 0x7f)
	} else if remainder > 0x7f {
		cxt.writeByte(uint8(remainder>>7)&0x7f + 0x80)
		cxt.writeByte(uint8(remainder>>0) & 0x7f)
	} else {
		cxt.writeByte(uint8(remainder))
	}

	return nil
}

func (cxt *Decoder) readDateAmf3() uint32 {
	ref := cxt.ReadUint29()

	if cxt.errored() {
		return 0
	}

	// Check the low bit to see if this is a reference
	if (ref & REFERENCE_BIT) == 0 {
		index := int(ref >> 1)
		if index >= len(cxt.stringTable) {
			cxt.saveError(errors.New(fmt.Sprintf("Invalid string index: %d", index)))
			return 0
		}

		if v, ok := cxt.objectTable[index].(uint32); ok {
			return v
		}

		return 0
	}

	ms := cxt.ReadFloat64()
	// TODO: timezone_offset
	result := uint32(ms / 1000.0)
	cxt.storeObjectInTable(result)
	return result
}

func (cxt *Decoder) readStringAmf3() string {
	ref := cxt.ReadUint29()

	if cxt.errored() {
		return ""
	}

	// Check the low bit to see if this is a reference
	if (ref & REFERENCE_BIT) == 0 {
		index := int(ref >> 1)
		if index >= len(cxt.stringTable) {
			cxt.saveError(errors.New(fmt.Sprintf("Invalid string index: %d", index)))
			return ""
		}

		return cxt.stringTable[index]
	}

	length := int(ref >> 1)

	if length == 0 {
		return ""
	}

	str := cxt.ReadStringKnownLength(length)
	cxt.stringTable = append(cxt.stringTable, str)

	return str
}

func (cxt *Encoder) WriteStringAmf3(s string) error {
	length := len(s)
	if length == 0 {
		return cxt.writeByte(0x01)
	}

	// TODO: Support outgoing string references.

	cxt.WriteUint29(uint32((length << 1) | 0x01))

	cxt.stream.Write([]byte(s))

	return nil
}

func (cxt *Decoder) readObjectAmf3() interface{} {

	ref := cxt.ReadUint29()

	if cxt.errored() {
		return nil
	}

	// Check the low bit to see if this is a reference
	if (ref & REFERENCE_BIT) == 0 {
		index := int(ref >> 1)
		if index >= len(cxt.objectTable) {
			cxt.saveError(errors.New(fmt.Sprintf("Invalid object index: %d", index)))
			return nil
		}
		return cxt.objectTable[index]
	}

	class := cxt.readClassDefinitionAmf3(ref)

	object := AvmObject{}
	object.Class = class

	// For an anonymous class, just return a map[string] interface{}
	if object.Class.Name == "" {
		if object.StaticFields == nil {
			object.StaticFields = make(map[string]interface{})
		}
		result := make(map[string]interface{})
		for _, prop := range class.Properties {
			value := cxt.ReadValueAmf3()
			object.StaticFields[prop] = value
		}
		if class.Dynamic {
			for {
				name := cxt.readStringAmf3()
				if name == "" {
					break
				}
				value := cxt.ReadValueAmf3()
				result[name] = value
			}
		}
		return result
	}

	object.DynamicFields = make(map[string]interface{})

	// Store the object in the table before doing any decoding.
	cxt.storeObjectInTable(&object)

	// Read static fields
	object.StaticFields = make(map[string]interface{}, len(class.Properties))
	for i := 0; i < len(class.Properties); i++ {
		v := class.Properties[i]
		value := cxt.ReadValueAmf3()
		if value == ExtObj {
			i = i - 1
			continue
		}
		object.StaticFields[v] = value
	}

	//fmt.Printf("static fields = %v\n", object.StaticFields)
	//fmt.Printf("static properties = %v\n", class.Properties)

	if class.Dynamic {
		// Parse dynamic fields
		for {
			name := cxt.readStringAmf3()
			if name == "" {
				break
			}

			value := cxt.ReadValueAmf3()
			object.DynamicFields[name] = value
		}
	}

	// If this type is registered, then unpack this result into an instance of the type.
	// TODO: This could be faster if we didn't create an intermediate AvmObject.
	goType, foundGoType := cxt.typeMap[class.Name]

	if foundGoType {
		result := reflect.Indirect(reflect.New(goType))
		//for i, v := range class.Properties{
		for i := 0; i < len(class.Properties); i++ {
			v := class.Properties[i]
			value := reflect.ValueOf(object.StaticFields[v])
			fieldName := class.Properties[i]
			// The Go type will have field names with capital letters
			fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
			field := result.FieldByName(fieldName)
			fmt.Printf("Attempting to write %v to field %v\n", object.StaticFields[v],
				class.Properties[i])
			field.Set(value)
		}
		return result.Interface()
	}

	if len(class.Properties) == 0 { // patch for resp body content
		return ExtObj
	}
	return object
}

func (cxt *Encoder) writeObjectAmf3(value interface{}) error {

	fmt.Printf("writeObjectAmf3 attempting to write a value of type %s\n",
		reflect.ValueOf(value).Type().Name())

	return nil
}

func (cxt *Encoder) writeAvmObject3(value *AvmObject) error {
	// TODO: Support outgoing object references.

	// writeClassDefinitionAmf3 will also write the ref section.
	cxt.writeClassDefinitionAmf3(value.Class)

	return nil
}

func (cxt *Encoder) writeReflectedMapAmf3(value reflect.Value) error {
	if value.Kind() != reflect.Map {
		return errors.New("writeReflectedMapAmf3 called with non-struct value")
	}

	ref := 0x00
	numFields := 0
	ref += numFields << 4
	final_ref := ref |
		0x02<<2 |
		REFERENCE_BIT<<1 |
		REFERENCE_BIT

	cxt.WriteUint29(uint32(final_ref))
	cxt.WriteUint8(0x01) //alias.anonymous
	mk := value.MapKeys()
	for _, k := range mk {
		cxt.WriteStringAmf3(k.String())

		if val, ok := value.MapIndex(k).Interface().(string); ok {
			if val == "" {
				cxt.WriteUint8(amf3_nullType)
				continue
			}
		}
		cxt.WriteValueAmf3(value.MapIndex(k).Interface())
	}
	cxt.WriteUint8(0x01)
	return nil
}

func (cxt *Encoder) writeReflectedStructAmf3(value reflect.Value) error {

	if value.Kind() != reflect.Struct {
		return errors.New("writeReflectedStructAmf3 called with non-struct value")
	}

	ref := 0x00
	numFields := value.Type().NumField()
	ref += numFields << 4
	final_ref := ref |
		0x02<<2 |
		REFERENCE_BIT<<1 |
		REFERENCE_BIT

	cxt.WriteUint29(uint32(final_ref))

	//cxt.WriteStringAmf3(value.Type().Name())
	if value.Type().Name() == "FlexRemotingMessage" {
		cxt.WriteStringAmf3("flex.messaging.messages.RemotingMessage")
	}
	// Class name
	//cxt.WriteStringAmf3(value.Type().Name())
	//fmt.Printf("wrote class name = %s\n", value.Type().Name())

	// Property names
	for i := 0; i < numFields; i++ {
		structField := value.Type().Field(i)
		//xxname:=structField.Name
		cxt.WriteStringAmf3(lowerFirst(structField.Name))
		//fmt.Printf("wrote field name = %s\n", structField.Name)
	}

	// Property values
	for i := 0; i < numFields; i++ {
		if val, ok := value.Field(i).Interface().(string); ok {
			if val == "" {
				cxt.WriteUint8(amf3_nullType)
				continue
			}
		}
		cxt.WriteValueAmf3(value.Field(i).Interface())
	}

	cxt.WriteUint8(0x01)
	return nil
}

func (cxt *Decoder) readClassDefinitionAmf3(ref uint32) *AvmClass {
	// Check for a reference to an existing class definition
	if (ref & 2) == 0 {
		return cxt.classTable[int(ref>>2)]
	}

	// Parse a class definition
	className := cxt.readStringAmf3()

	externalizable := ref&4 != 0
	dynamic := ref&8 != 0
	propertyCount := ref >> 4

	class := AvmClass{className, externalizable, dynamic, make([]string, propertyCount)}

	// Property names
	for i := uint32(0); i < propertyCount; i++ {
		class.Properties[i] = cxt.readStringAmf3()
	}

	// Save the new class in the loopup table
	cxt.classTable = append(cxt.classTable, &class)

	return &class
}

func (cxt *Encoder) writeClassDefinitionAmf3(class *AvmClass) {
	// TODO: Support class references
	ref := uint32(0x2)

	if class.Externalizable {
		ref += 0x4
	}
	if class.Dynamic {
		ref += 0x8
	}

	ref += uint32(len(class.Properties) << 4)

	cxt.WriteUint29(ref)

	cxt.WriteStringAmf3(class.Name)

	// Property names
	for _, name := range class.Properties {
		cxt.WriteStringAmf3(name)
	}
}

func (cxt *Decoder) readArrayAmf3() interface{} {
	ref := cxt.ReadUint29()

	if cxt.errored() {
		return nil
	}

	// Check the low bit to see if this is a reference
	if (ref & 1) == 0 {
		index := int(ref >> 1)
		if index >= len(cxt.objectTable) {
			cxt.saveError(errors.New(fmt.Sprintf("Invalid array reference: %d", index)))
			return nil
		}

		return cxt.objectTable[index]
	}

	elementCount := int(ref >> 1)

	// Read name-value pairs, if any.
	key := cxt.readStringAmf3()

	// No name-value pairs, return a flat Go array.
	if key == "" {
		result := make([]interface{}, elementCount)
		cxt.storeObjectInTable(result)
		for i := 0; i < elementCount; i++ {
			result[i] = cxt.ReadValueAmf3()
		}
		return result
	}

	result := &AvmArray{}
	result.fields = make(map[string]interface{})

	// Store the object in the table before doing any decoding.
	cxt.storeObjectInTable(result)

	for key != "" {
		result.fields[key] = cxt.ReadValueAmf3()
		key = cxt.readStringAmf3()
	}

	// Read dense elements
	result.elements = make([]interface{}, elementCount)
	for i := 0; i < elementCount; i++ {
		result.elements[i] = cxt.ReadValueAmf3()
	}

	return result
}

func (cxt *Encoder) writeReflectedArrayAmf3(value reflect.Value) error {

	elementCount := value.Len()

	// TODO: Support outgoing array references
	ref := (elementCount << 1) + 1

	cxt.WriteUint29(uint32(ref))

	// Write an empty key since this is just a flat array.
	//cxt.WriteStringAmf3("")
	cxt.WriteUint8(0x01)

	for i := 0; i < elementCount; i++ {
		cxt.WriteValueAmf3(value.Index(i).Interface())
	}
	return nil
}

func (cxt *Encoder) writeFlatArrayAmf3(value []interface{}) error {
	elementCount := len(value)

	// TODO: Support outgoing array references
	ref := (elementCount << 1) + 1

	cxt.WriteUint29(uint32(ref))

	// Write an empty key since this is just a flat array.
	cxt.WriteStringAmf3("")

	// Write dense elements
	for i := 0; i < elementCount; i++ {
		cxt.WriteValueAmf3(value[i])
	}
	return nil
}

func (cxt *Encoder) writeMixedArray3(value *AvmArray) error {
	elementCount := len(value.elements)

	// TODO: Support outgoing array references
	ref := (elementCount << 1) + 1

	cxt.WriteUint29(uint32(ref))

	// Write fields
	for k, v := range value.fields {
		cxt.WriteStringAmf3(k)
		cxt.WriteValueAmf3(v)
	}

	// Write a null name to indicate the end of fields.
	cxt.WriteStringAmf3("")

	// Write dense elements
	for i := 0; i < elementCount; i++ {
		cxt.WriteValueAmf3(value.elements[i])
	}
	return nil
}

func (cxt *Decoder) ReadValue() interface{} {
	if cxt.IsAMF3 {
		return cxt.ReadValueAmf3()
	}

	// return cxt.ReadVal()
	return cxt.readValueAmf0()
}

func (cxt *Decoder) readValueAmf0() interface{} {

	typeMarker := cxt.ReadByte()

	if cxt.errored() {
		return nil
	}

	// Most AMF0 types are not yet supported.

	// Type markers
	switch typeMarker {
	case amf0_numberType:
		return cxt.ReadFloat64()
	case amf0_booleanType:
		val := cxt.ReadUint8()
		return val != 0
	case amf0_stringType:
		_, v := cxt.ReadString()
		return v
	case amf0_objectType:
		result := map[string]interface{}{}
		for true {
			c1 := cxt.ReadByte()
			c2 := cxt.ReadByte()
			length := int(c1)<<8 + int(c2)
			name := cxt.ReadStringKnownLength(length)
			result[name] = cxt.readValueAmf0()
		}
		return result

	case amf0_movieClipType:
		fmt.Printf("Movie clip type not supported")
	case amf0_nullType:
		return nil
	case amf0_undefinedType:
		return nil
	case amf0_referenceType:
	case amf0_ecmaArrayType:
		return cxt.readArrayAmf0()
	case amf0_objectEndType:
		return "objectEnd"
	case amf0_strictArrayType:
	case amf0_dateType:
	case amf0_longStringType:
	case amf0_unsupporedType:
	case amf0_recordsetType:
	case amf0_xmlObjectType:
	case amf0_typedObjectType:
	case amf0_avmPlusObjectType:
		return cxt.ReadValueAmf3()
	}

	fmt.Printf("AMF0 type marker was not supported: %d", typeMarker)
	return nil
}

func (cxt *Decoder) ReadValueAmf3() interface{} {
	// Read type marker
	typeMarker := cxt.ReadByte()

	if cxt.errored() {
		return nil
	}

	// Flash Player 9 will sometimes wrap data as an AMF0 value, which just means that
	// there might be an additional type code (amf0_avmPlusObjectType), which we can
	// unambiguously ignore here.

	if typeMarker == amf0_avmPlusObjectType {
		typeMarker = cxt.ReadByte()
		if cxt.errored() {
			return nil
		}
	}

	switch typeMarker {
	case amf3_nullType, amf3_undefinedType:
		return nil
	case amf3_falseType:
		return false
	case amf3_trueType:
		return true
	case amf3_integerType:
		return cxt.ReadUint29()
	case amf3_doubleType:
		return cxt.ReadFloat64()
	case amf3_stringType:
		return cxt.readStringAmf3()
	case amf3_xmlType:
		// TODO
	case amf3_dateType:
		return cxt.readDateAmf3()
		// TODO
	case amf3_objectType:
		return cxt.readObjectAmf3()
	case amf3_avmPlusXmlType:
		// TODO
	case amf3_byteArrayType:
		// TODO
	case amf3_arrayType:
		return cxt.readArrayAmf3()
	}

	cxt.saveError(errors.New("AMF3 type marker was not supported"))
	return nil
}

func (cxt *Encoder) WriteValueAmf3(value interface{}) error {
	if value == nil {
		return cxt.writeByte(amf3_nullType)
	}

	return cxt.writeReflectedValueAmf3(reflect.ValueOf(value))
}

func (cxt *Encoder) writeReflectedValueAmf3(value reflect.Value) error {
	switch value.Kind() {
	case reflect.String:
		cxt.writeByte(amf3_stringType)
		str, _ := value.Interface().(string)
		return cxt.WriteStringAmf3(str)
	case reflect.Bool:
		if value.Bool() == false {
			return cxt.writeByte(amf3_falseType)
		} else {
			return cxt.writeByte(amf3_trueType)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		cxt.writeByte(amf3_integerType)
		return cxt.WriteUint29(uint32(value.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		cxt.writeByte(amf3_integerType)
		return cxt.WriteUint29(uint32(value.Uint()))
	case reflect.Float32, reflect.Float64:
		cxt.writeByte(amf3_doubleType)
		return cxt.WriteFloat64(value.Float())
	case reflect.Array, reflect.Slice:
		cxt.writeByte(amf3_arrayType)
		return cxt.writeReflectedArrayAmf3(value)
	case reflect.Struct:
		cxt.writeByte(amf3_objectType)
		return cxt.writeReflectedStructAmf3(value)
	case reflect.Map:
		cxt.writeByte(amf3_objectType)
		return cxt.writeReflectedMapAmf3(value)
	}

	return errors.New(fmt.Sprintf("writeReflectedArrayAmf3 doesn't support kind: %v",
		value.Kind().String()))
}

func lowerFirst(s string) string {
	a := []byte(s)
	a[0] += 32
	s = string(a)
	return s
}

// ObjectProperty amf 对象属性
type ObjectProperty struct {
	Name  string
	Value interface{}
}

// EcmaArray 表示 TypeEcmaArray 类型存储的值
type EcmaArray []ObjectProperty

func (cxt *Decoder) readArrayAmf0() interface{} {

	ref := cxt.ReadUint32()
	if cxt.errored() {
		return nil
	}
	if int(ref) == 0 {
		return nil
	}
	// elementCount := int(ref)
	// result := make(EcmaArray, 0, elementCount)

	// for {
	// 	var elem ObjectProperty
	// 	if len, elem.Name := cxt.ReadString(); len == 0 {
	// 		return
	// 	}
	// 	if elem.Value := cxt.readValueAmf0(); elem.Value == nil {
	// 		return
	// 	}
	// 	if elem.Value == amf0_objectEndType && len(elem.Name) == 0 {
	// 		break
	// 	}
	// 	result = appned(result, elem)

	// }

	// return

	// if (ref & 1) == 0 {
	// 	index := int(ref >> 1)
	// 	if index >= len(cxt.objectTable) {
	// 		cxt.saveError(errors.New(fmt.Sprintf("Invalid array reference: %d", index)))
	// 		return nil
	// 	}
	// }

	// elementCount := int(ref)
	// result := make([]interface{}, elementCount)
	// cxt.storeObjectInTable(result)

	// for i := 0; i < elementCount; i++ {
	// 	cxt.
	// 	result[i] = cxt.readValueAmf0()
	// }
	// return result

	// for i := 0; i < elementCount; i++ {
	// _, key := cxt.ReadString()
	// // result[i] = cxt.readValueAmf0()
	// // }
	// if key == "" {
	// 	result := make([]interface{}, elementCount)
	// 	cxt.storeObjectInTable(result)
	// 	for i := 0; i < elementCount; i++ {
	// 		result[i] = cxt.readValueAmf0()
	// 	}
	// 	return result
	// }

	// result := &AvmArray{}
	result := make(map[string]interface{})
	_, key := cxt.ReadString()
	// Store the object in the table before doing any decoding.
	cxt.storeObjectInTable(result)

	for key != "" {
		result[key] = cxt.readValueAmf0()
		_, key = cxt.ReadString()
	}
	if key == "" && cxt.readValueAmf0() == "objectEnd" {
		return result
	}
	// result.elements = make([]interface{}, elementCount)
	// for i := 0; i < elementCount; i++ {
	// 	result.elements[i] = cxt.readValueAmf0()
	// }
	// return nil

	return result

	// 	return cxt.objectTable[index]
	// }
	// elementCount := int(ref >> 1)

	// associativeCount := binary.BigEndian.Uint32(decoder.u32[:])
	// obj, err := decoder.readObject(r)
	// if err != nil {
	// 	return nil, err
	// }
	// if uint32(len(obj)) != associativeCount {
	// 	return nil, errors.New("ECMAArray Count error")
	// }
	// decoder.refObjects = append(decoder.refObjects, obj)
	// return obj, nil
}

// func storeObjectAmf0(obj interface{}){

// case amf0_objectType:
// 	result := map[string]interface{}{}
// 	for true {
// 		c1 := cxt.ReadByte()
// 		c2 := cxt.ReadByte()
// 		length := int(c1)<<8 + int(c2)
// 		name := cxt.ReadStringKnownLength(length)
// 		result[name] = cxt.readValueAmf0()
// 	}
// 	return result

// 	cxt.objectTable = append(cxt.objectTable, obj)
// }

// func (cxt *Decoder) readArrayAmf0() interface{} {
// 	ref := cxt.ReadUint29()

// 	if cxt.errored() {
// 		return nil
// 	}

// 	// Check the low bit to see if this is a reference
// 	if (ref & 1) == 0 {
// 		index := int(ref >> 1)
// 		if index >= len(cxt.objectTable) {
// 			cxt.saveError(errors.New(fmt.Sprintf("Invalid array reference: %d", index)))
// 			return nil
// 		}

// 		return cxt.objectTable[index]
// 	}

// 	elementCount := int(ref >> 1)

// 	// Read name-value pairs, if any.
// 	_, key := cxt.ReadString()

// 	// No name-value pairs, return a flat Go array.
// 	if key == "" {
// 		result := make([]interface{}, elementCount)
// 		cxt.storeObjectInTable(result)
// 		for i := 0; i < elementCount; i++ {
// 			result[i] = cxt.ReadValueAmf0()
// 		}
// 		return result
// 	}

// 	result := &AvmArray{}
// 	result.fields = make(map[string]interface{})

// 	// Store the object in the table before doing any decoding.
// 	cxt.storeObjectInTable(result)

// 	for key != "" {
// 		result.fields[key] = cxt.ReadValueAmf0()
// 		_, key = cxt.ReadString()
// 	}

// 	// Read dense elements
// 	result.elements = make([]interface{}, elementCount)
// 	for i := 0; i < elementCount; i++ {
// 		result.elements[i] = cxt.ReadValueAmf0()
// 	}

// 	return result
// }
