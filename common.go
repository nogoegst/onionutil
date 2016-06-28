// common.go - commonly used functions for onions
//
// To the extent possible under law, Ivan Markin waived all copyright
// and related or neighboring rights to this module of onionutil, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package onionutil

import (
    "fmt"
    "crypto"
    "crypto/rsa"
    "crypto/sha1"
    "encoding/base32"
    "encoding/base64"
    "encoding/binary"
    "time"
    "strings"
    "bytes"
    "strconv"
    "reflect"
    "onionutil/pkcs1"

)


const (
	PublicationTimeFormat = "2006-01-02 15:04:05"
	NTorOnionKeySize = 32
)

const HashType = crypto.SHA1

func Hash(data []byte) (hash []byte) {
    h := sha1.New()
    h.Write(data)
    hash = h.Sum(nil)
    return hash
}
func RSAPubkeyHash(pk *rsa.PublicKey) (derHash []byte, err error) {
    der, err := pkcs1.EncodePublicKeyDER(pk)
    if err != nil {
        return
    }
    derHash = Hash(der)
    return derHash, err
}

func CalcPermanentId(pk *rsa.PublicKey) (permId []byte, err error) {
    derHash, err := RSAPubkeyHash(pk)
    if err != nil {
	return
    }
    permId = derHash[:10]
    return
}

/* XXX: here might be an error for new ed25519 addresses (! mod 5bits=0) */
func Base32Encode(binary []byte) (string) {
    hb32 := base32.StdEncoding.EncodeToString(binary)
    return strings.ToLower(hb32)
}

func Base32Decode(b32 string) (binary []byte, err error) {
    binary, err = base32.StdEncoding.DecodeString(strings.ToUpper(b32))
    return binary, err
}

func Base64Decode(b64 []byte) (binary []byte, n int, err error) {
	binary = make([]byte, base64.RawStdEncoding.DecodedLen(len(b64)))
        n, err = base64.StdEncoding.Decode(binary, b64)
	if err != nil {
		n, err = base64.RawStdEncoding.Decode(binary, b64)
	}
	return binary, n, err
}

// OnionAddress returns the Tor Onion Service address corresponding to a given
// rsa.PublicKey.
func OnionAddress(pk *rsa.PublicKey) (onion_address string, err error) {
    perm_id, err := CalcPermanentId(pk)
    if err != nil {
        return onion_address, err
    }
    onion_address = Base32Encode(perm_id)
    return onion_address, err
}

func InetPortFromByteString(str []byte) (port uint16, err error) {
	p, err := strconv.ParseUint(string(str), 10, 16)
	return uint16(p), err
}


type Platform struct {
    SoftwareName string
    SoftwareVersion string
    Name string
}

func ParseRouterSoftwareVersion(data [][]byte) (platform Platform, err error) {
	if len(data) < 2 {
		return platform, fmt.Errorf("Software version is too short")
	}
	platform.SoftwareName = string(bytes.Join(data[:len(data)-1], []byte(" ")))
	platform.SoftwareVersion = string(data[len(data)-1])
	return platform, err
}

func ParsePlatformEntry(platformE [][]byte) (platform Platform, err error) {
    /* XXX: lil crafty */
    var onIndexes []int
    for i, word := range platformE {
	if reflect.DeepEqual(word, []byte("on")) {
		onIndexes = append(onIndexes, i)
	}
    }
    if len(onIndexes) != 1 {
	return platform, fmt.Errorf("Platform string contains not exacly one \" on \"")
    }
    platform, err = ParseRouterSoftwareVersion(platformE[:onIndexes[0]])
    if err != nil {
	return platform, nil
    }
    platform.Name = string(bytes.Join(platformE[onIndexes[0]+1:], []byte(" ")))
    return platform, err
}




type ExitPolicy struct {
	Reject []string
	Accept []string
}

type Exit6Policy struct {
	Accept	bool
	PortList	[]string
}

func ParsePolicy(entry [][]byte) (policy Exit6Policy, err error) {
	switch string(entry[0]) {
		case "reject":
			policy.Accept = false
		case "accept":
			policy.Accept = true
		default:
			return policy, fmt.Errorf("Policy is not recognized")
	}

	for _, port := range entry[1:] {
		policy.PortList =
			append(policy.PortList, string(port))
	}
	return policy, err
}


type Bandwidth struct {
	Average uint64
	Burst	uint64
	Observed	uint64
}

func ParseBandwidthEntry(bandwidthE [][]byte) (bandwidth Bandwidth, err error) {
	if len(bandwidthE) != 3 {
		return bandwidth, fmt.Errorf("Bandwidth entry length is not equal 4")
	}
	average, err := strconv.ParseUint(string(bandwidthE[0]), 10, 64);
	if err != nil {
		return bandwidth, err
	}
	burst, err := strconv.ParseUint(string(bandwidthE[1]), 10, 64);
	if err !=nil {
		return bandwidth, err
	}
	observed, err := strconv.ParseUint(string(bandwidthE[2]), 10, 64);
	if err != nil {
		return bandwidth, err
	}
	bandwidth = Bandwidth{Average: average, Burst: burst, Observed: observed}
	return bandwidth, err
}
const Ed25519PubkeySize		= 32
const Ed25519SignatureSize	= 64
const Curve25519PubkeySize	= 32
const RSAPubkeySize		= 128
const RSASignatureSize		= 128

type Ed25519Pubkey	[Ed25519PubkeySize]byte
type Ed25519Signature	[Ed25519SignatureSize]byte
type Curve25519Pubkey	[Curve25519PubkeySize]byte
type RSASignature	[RSASignatureSize]byte


type ExtType byte
type Extension struct {
	Type	ExtType
	Flags	byte
	Data	[]byte
}

/*
type CertKeyType byte

const (
	RESERVED0 CertKeyType	= 0x00
	RESERVED1		= 0x01
	RESERVED2		= 0x02
	RESERVED3		= 0x03
	
*/

type Certificate struct {
	Version	uint8
	CertType		byte
	ExpirationDate	time.Time
	CertKeyType	byte
	CertifiedKey	Ed25519Pubkey
	NExtensions	uint8
	Extensions	map[ExtType]Extension
	Signature	Ed25519Signature
	PubkeySign	bool
}

func ParseCertFromBytes(binCert []byte) (cert Certificate, err error) {
	i := 0 /* Index */
	cert.Version = uint8(binCert[i])
	i+=1
	cert.CertType = binCert[i]
	i+=1
	expirationHours := binary.BigEndian.Uint32(binCert[i:i+4])
	i+=4
	expirationDuration := time.Duration(expirationHours)*time.Hour
	expirationIntDate := int64(expirationDuration.Seconds())
	cert.ExpirationDate = time.Unix(expirationIntDate,0)
	cert.CertKeyType = binCert[i]
	i+=1
        copy(cert.CertifiedKey[:], binCert[i:i+Ed25519PubkeySize])
	i+=Ed25519PubkeySize
	cert.NExtensions = uint8(binCert[i])
	i+=1
	cert.Extensions = make(map[ExtType]Extension)
	for e := 0; e<int(cert.NExtensions); e++ {
		var extension Extension
		extLength := int(binary.BigEndian.Uint16(binCert[i:i+2]))
		i+=2
		extension.Type = ExtType(binCert[i])
		i+=1
		extension.Flags = binCert[i]
		i+=1
		extension.Data = binCert[i:i+extLength]
		i+=extLength
		/* We assume that there are no duplicates by ExtType */
		cert.Extensions[extension.Type] = extension
	}
	copy(cert.Signature[:], binCert[i:i+Ed25519SignatureSize])
	i+=Ed25519SignatureSize
	return
}

