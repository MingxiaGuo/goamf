package amf

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func decodeMessageBundleFromHex(s string) (*MessageBundle, error) {
	requestBinary, _ := hex.DecodeString(s)
	reader := bytes.NewBuffer(requestBinary)

	return DecodeMessageBundle(reader)
}

func TestDecodeExampleRequest(t *testing.T) {

	// Bits for an example request, generated by the Pinta tool:
	const exampleRequest = "00030000000100046e756c6c00022f37000001090a0000000" +
		"1110a81134f666c65782e6d6573736167696e672e6d657373616765732e52656d6f7" +
		"4696e674d6573736167650d736f75726365136f7065726174696f6e136d657373616" +
		"765496411636c69656e7449641574696d65546f4c6976650f6865616465727313746" +
		"96d657374616d7009626f64791764657374696e6174696f6e060f736572766963650" +
		"60d6d6574686f64064936353732364339332d393243322d443132452d444637392d3" +
		"14330343138453135363944064939646534396137642d333736372d343264362d613" +
		"537652d32313137373263646633306404000a0b01154453456e64706f696e7401094" +
		"453496406076e696c010400090101060d616d66706870"

	bundle, err := decodeMessageBundleFromHex(exampleRequest)

	if err != nil {
		t.Errorf("DecodeMessageBundle returned error: %v", err)
		return
	}

	if bundle.AmfVersion != 3 {
		t.Errorf("Wrong amfVersion: %d", bundle.AmfVersion)
	}
	if len(bundle.Headers) != 0 {
		t.Errorf("Wrong number of headers: %d", len(bundle.Headers))
	}
	if len(bundle.Messages) != 1 {
		t.Errorf("Wrong number of messages: %d", len(bundle.Messages))
		return
	}
	message := bundle.Messages[0]
	if message.TargetUri != "null" {
		t.Errorf("Wrong target uri: %s", message.TargetUri)
	}
	if message.ResponseUri != "/7" {
		t.Errorf("Wrong response uri: %s", message.ResponseUri)
	}

	bodyArray, ok := message.Body.([]interface{})

	if !ok {
		t.Errorf("Couldn't cast to array: %v\n", message.Body)
		return
	}

	frm, ok := bodyArray[0].(FlexRemotingMessage)

	if !ok {
		t.Errorf("Couldn't cast to FlexRemotingMessage: %v\n", bodyArray[0])
		return
	}

	if frm.Timestamp != 0 {
		t.Error("Wrong timestamp")
	}
	if frm.TimeToLive != 0 {
		t.Error("Wrong timeToLive")
	}
	if frm.Source != "service" {
		t.Error("Wrong source")
	}
	if frm.Operation != "method" {
		t.Error("Wrong operation")
	}
	if frm.MessageId != "65726C93-92C2-D12E-DF79-1C0418E1569D" {
		t.Error("Wrong messageId")
	}
	if frm.ClientId != "9de49a7d-3767-42d6-a57e-211772cdf30d" {
		t.Error("Wrong clientId")
	}
	if frm.Destination != "amfphp" {
		t.Error("Wrong destination")
	}
}

func TestDecodeExampleRequest2(t *testing.T) {

	// Another example request generated by the Pinta tool. This one
	// has arguments in it.
	const exampleRequest = "00030000000100046e756c6c00022f340000012a0a0000000" +
		"1110a81134f666c65782e6d6573736167696e672e6d657373616765732e52656d6f7" +
		"4696e674d6573736167650d736f75726365136f7065726174696f6e136d657373616" +
		"765496411636c69656e7449641574696d65546f4c6976650f6865616465727313746" +
		"96d657374616d7009626f64791764657374696e6174696f6e06136d7953657276696" +
		"36506116d794d6574686f64064939314233453136372d443335412d373542462d363" +
		"245362d314345393842433432434446064964333462613438662d666438362d34313" +
		"5392d383035662d33363730656438343733343004000a0b01154453456e64706f696" +
		"e7401094453496406076e696c0104000907010609747275650603350a05096361747" +
		"30405096e616d65060753616d01060d616d66706870"

	bundle, err := decodeMessageBundleFromHex(exampleRequest)

	if err != nil {
		t.Errorf("DecodeMessageBundle returned error: %v", err)
		return
	}

	bodyArray, ok := bundle.Messages[0].Body.([]interface{})

	if !ok {
		t.Error("Couldn't cast message body to array")
		return
	}

	frm, ok := bodyArray[0].(FlexRemotingMessage)

	if !ok {
		t.Errorf("Couldn't cast to FlexRemotingMessage: %v\n", bodyArray[0])
		return
	}

	if frm.Source != "myService" {
		t.Error("Wrong source")
	}
	if frm.Operation != "myMethod" {
		t.Error("Wrong operation")
	}
	if frm.MessageId != "91B3E167-D35A-75BF-62E6-1CE98BC42CDF" {
		t.Errorf("Wrong messageId: %v", frm.MessageId)
	}
	if frm.ClientId != "d34ba48f-fd86-4159-805f-3670ed847340" {
		t.Errorf("Wrong clientID: %v", frm.ClientId)
	}
	if frm.Destination != "amfphp" {
		t.Error("Wrong destination")
	}

	bodyStr := fmt.Sprintf("%v", frm.Body)
	if bodyStr != "[true 5 map[name:Sam cats:5]]" {
		t.Errorf("Wrong message body: %s", bodyStr)
	}
}