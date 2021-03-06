// File contains Modify functionality
//
// https://tools.ietf.org/html/rfc4511
//
// ModifyRequest ::= [APPLICATION 6] SEQUENCE {
//      object          LDAPDN,
//      changes         SEQUENCE OF change SEQUENCE {
//           operation       ENUMERATED {
//                add     (0),
//                delete  (1),
//                replace (2),
//                ...  },
//           modification    PartialAttribute } }
//
// PartialAttribute ::= SEQUENCE {
//      type       AttributeDescription,
//      vals       SET OF value AttributeValue }
//
// AttributeDescription ::= LDAPString
//                         -- Constrained to <attributedescription>
//                         -- [RFC4512]
//
// AttributeValue ::= OCTET STRING
//

package ldap

import (
	"errors"
	"log"

	"github.com/gostores/encoding/asn1"
)

// Change operation choices
const (
	AddAttribute     = 0
	DeleteAttribute  = 1
	ReplaceAttribute = 2
)

// PartialAttribute for a ModifyRequest as defined in https://tools.ietf.org/html/rfc4511
type PartialAttribute struct {
	// Type is the type of the partial attribute
	Type string
	// Vals are the values of the partial attribute
	Vals []string
}

func (p *PartialAttribute) encode() *asn1.Packet {
	seq := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSequence, nil, "PartialAttribute")
	seq.AppendChild(asn1.NewString(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagOctetString, p.Type, "Type"))
	set := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSet, nil, "AttributeValue")
	for _, value := range p.Vals {
		set.AppendChild(asn1.NewString(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagOctetString, value, "Vals"))
	}
	seq.AppendChild(set)
	return seq
}

// ModifyRequest as defined in https://tools.ietf.org/html/rfc4511
type ModifyRequest struct {
	// DN is the distinguishedName of the directory entry to modify
	DN string
	// AddAttributes contain the attributes to add
	AddAttributes []PartialAttribute
	// DeleteAttributes contain the attributes to delete
	DeleteAttributes []PartialAttribute
	// ReplaceAttributes contain the attributes to replace
	ReplaceAttributes []PartialAttribute
}

// Add inserts the given attribute to the list of attributes to add
func (m *ModifyRequest) Add(attrType string, attrVals []string) {
	m.AddAttributes = append(m.AddAttributes, PartialAttribute{Type: attrType, Vals: attrVals})
}

// Delete inserts the given attribute to the list of attributes to delete
func (m *ModifyRequest) Delete(attrType string, attrVals []string) {
	m.DeleteAttributes = append(m.DeleteAttributes, PartialAttribute{Type: attrType, Vals: attrVals})
}

// Replace inserts the given attribute to the list of attributes to replace
func (m *ModifyRequest) Replace(attrType string, attrVals []string) {
	m.ReplaceAttributes = append(m.ReplaceAttributes, PartialAttribute{Type: attrType, Vals: attrVals})
}

func (m ModifyRequest) encode() *asn1.Packet {
	request := asn1.Encode(asn1.ClassApplication, asn1.TypeConstructed, ApplicationModifyRequest, nil, "Modify Request")
	request.AppendChild(asn1.NewString(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagOctetString, m.DN, "DN"))
	changes := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSequence, nil, "Changes")
	for _, attribute := range m.AddAttributes {
		change := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSequence, nil, "Change")
		change.AppendChild(asn1.NewInteger(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagEnumerated, uint64(AddAttribute), "Operation"))
		change.AppendChild(attribute.encode())
		changes.AppendChild(change)
	}
	for _, attribute := range m.DeleteAttributes {
		change := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSequence, nil, "Change")
		change.AppendChild(asn1.NewInteger(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagEnumerated, uint64(DeleteAttribute), "Operation"))
		change.AppendChild(attribute.encode())
		changes.AppendChild(change)
	}
	for _, attribute := range m.ReplaceAttributes {
		change := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSequence, nil, "Change")
		change.AppendChild(asn1.NewInteger(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagEnumerated, uint64(ReplaceAttribute), "Operation"))
		change.AppendChild(attribute.encode())
		changes.AppendChild(change)
	}
	request.AppendChild(changes)
	return request
}

// NewModifyRequest creates a modify request for the given DN
func NewModifyRequest(
	dn string,
) *ModifyRequest {
	return &ModifyRequest{
		DN: dn,
	}
}

// Modify performs the ModifyRequest
func (l *Conn) Modify(modifyRequest *ModifyRequest) error {
	packet := asn1.Encode(asn1.ClassUniversal, asn1.TypeConstructed, asn1.TagSequence, nil, "LDAP Request")
	packet.AppendChild(asn1.NewInteger(asn1.ClassUniversal, asn1.TypePrimitive, asn1.TagInteger, l.nextMessageID(), "MessageID"))
	packet.AppendChild(modifyRequest.encode())

	l.Debug.PrintPacket(packet)

	msgCtx, err := l.sendMessage(packet)
	if err != nil {
		return err
	}
	defer l.finishMessage(msgCtx)

	l.Debug.Printf("%d: waiting for response", msgCtx.id)
	packetResponse, ok := <-msgCtx.responses
	if !ok {
		return NewError(ErrorNetwork, errors.New("ldap: response channel closed"))
	}
	packet, err = packetResponse.ReadPacket()
	l.Debug.Printf("%d: got response %p", msgCtx.id, packet)
	if err != nil {
		return err
	}

	if l.Debug {
		if err := addLDAPDescriptions(packet); err != nil {
			return err
		}
		asn1.PrintPacket(packet)
	}

	if packet.Children[1].Tag == ApplicationModifyResponse {
		resultCode, resultDescription := getLDAPResultCode(packet)
		if resultCode != 0 {
			return NewError(resultCode, errors.New(resultDescription))
		}
	} else {
		log.Printf("Unexpected Response: %d", packet.Children[1].Tag)
	}

	l.Debug.Printf("%d: returning", msgCtx.id)
	return nil
}
