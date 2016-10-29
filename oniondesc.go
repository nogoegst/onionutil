// oniondesc.go - deal with onion service descriptors
//
// To the extent possible under law, Ivan Markin waived all copyright
// and related or neighboring rights to this module of onionutil, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package onionutil

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/nogoegst/onionutil/pkcs1"
	"github.com/nogoegst/onionutil/torparse"
)

type OnionDescriptor struct {
	DescID             []byte
	Version            int
	PermanentKey       *rsa.PublicKey
	SecretIDPart       []byte
	PublicationTime    time.Time
	ProtocolVersions   []int
	IntropointsBlock   []byte
	Signature          []byte
}

var(
	MinReplica = 0
	MaxReplica = 1
	DescVersion = 2
	ProtocolVersions = []int{2, 3}
)

// Initialize defaults
func (desc *OnionDescriptor) Update(replica int) (err error){
	/* v hardcoded values */
	desc.Version = DescVersion
	desc.ProtocolVersions = ProtocolVersions
	/* ^ hardcoded values */
	currentTime := time.Now().Unix()
	roundedCurrentTime := currentTime - currentTime%(60*60)
	desc.PublicationTime = time.Unix(roundedCurrentTime, 0)
	permID, err := CalcPermanentID(desc.PermanentKey)
	if err != nil {
		return err
	}
	if !(MinReplica <= replica || replica <= MaxReplica) {
		return fmt.Errorf("Replica is out of range")
	}
	desc.SecretIDPart = calcSecretID(permID, currentTime, byte(replica))
	desc.DescID = calcDescriptorID(permID, desc.SecretIDPart)
	return
}

// TODO return a pointer to descs not descs themselves?
func ParseOnionDescriptors(descsData []byte) (descs []OnionDescriptor, rest []byte) {
	docs, rest := torparse.ParseTorDocument(descsData)
	for _, doc := range docs {
		var desc OnionDescriptor
		if _, ok := doc["rendezvous-service-descriptor"]; !ok {
			log.Printf("Got a document that is not an onion service")
			continue
		} else {
			desc.DescID = doc["rendezvous-service-descriptor"].FJoined()
		}

		version, err := strconv.ParseInt(string(doc["version"].FJoined()), 10, 0)
		if err != nil {
			log.Printf("Error parsing descriptor version: %v", err)
			continue
		}
		desc.Version = int(version)

		permanentKey, _, err := pkcs1.DecodePublicKeyDER(doc["permanent-key"].FJoined())
		if err != nil {
			log.Printf("Decoding DER sequence of PulicKey has failed: %v.", err)
			continue
		}
		desc.PermanentKey = permanentKey
		desc.IntropointsBlock = doc["introduction-points"].FJoined()

		if len(doc["signature"][0]) < 1 {
			log.Printf("Empty signature")
			continue
		}
		desc.Signature = doc["signature"].FJoined()

		descs = append(descs, desc)
	}

	return descs, rest
}

func (desc *OnionDescriptor) Bytes() []byte {
	w := new(bytes.Buffer)
	permPubKeyDER, err := pkcs1.EncodePublicKeyDER(desc.PermanentKey)
	if err != nil {
		log.Fatalf("Cannot encode public key into DER sequence.")
	}
	fmt.Fprintf(w, "rendezvous-service-descriptor %s\n", Base32Encode(desc.DescID))
	fmt.Fprintf(w, "version %d\n", desc.Version)
	fmt.Fprintf(w, "permanent-key\n%s",
		pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY",
			Bytes: permPubKeyDER}))
	fmt.Fprintf(w, "secret-id-part %s\n",
		Base32Encode(desc.SecretIDPart))
	fmt.Fprintf(w, "publication-time %v\n",
		desc.PublicationTime.Format("2006-01-02 15:04:05"))
	var protoversions []string
	for _, v := range desc.ProtocolVersions {
		protoversions = append(protoversions, fmt.Sprintf("%d", v))
	}
	fmt.Fprintf(w, "protocol-versions %v\n",
		strings.Join(protoversions, ","))
	if len(desc.IntropointsBlock) > 0 {
		pemIntroBlock := &pem.Block{Type: "MESSAGE", Bytes: []byte(desc.IntropointsBlock)}
		fmt.Fprintf(w, "introduction-points\n%s", pem.EncodeToMemory(pemIntroBlock))
	}
	fmt.Fprintf(w, "signature\n")
	if len(desc.Signature) > 0 {
		pemSignature := pem.EncodeToMemory(&pem.Block{Type: "SIGNATURE", Bytes: desc.Signature})
		fmt.Fprintf(w, "%s", pemSignature)
	}
	return w.Bytes()
}

func (desc *OnionDescriptor) Sign(doSign func(digest []byte) ([]byte, error)) (error) {
	descDigest := Hash(desc.Bytes())
	signature, err := doSign(descDigest)
	if err != nil {
		return err
	}
	desc.Signature = signature
	return nil
}

func (desc *OnionDescriptor) VerifySignature() (error) {
	signature := desc.Signature
	desc.Signature = []byte{}
	descDigest := Hash(desc.Bytes())
	desc.Signature = signature
	return rsa.VerifyPKCS1v15(desc.PermanentKey, 0, descDigest, signature)
}

/* TODO: there is no `descriptor-cookie` now (because we need IP list encryption etc) */
func calcSecretID(permID []byte, currentTime int64, replica byte) (secretID []byte) {
	permIDByte := uint32(permID[0])

	timePeriodInt := (uint32(currentTime) + permIDByte*86400/256) / 86400
	var timePeriod = new(bytes.Buffer)
	binary.Write(timePeriod, binary.BigEndian, timePeriodInt)

	h := sha1.New()
	h.Write(timePeriod.Bytes())
	h.Write([]byte{replica})
	secretID = h.Sum(nil)
	return secretID
}

func calcDescriptorID(permID, secretID []byte) (descID []byte) {
	h := sha1.New()
	h.Write(permID)
	h.Write(secretID)
	descID = h.Sum(nil)
	return descID
}
