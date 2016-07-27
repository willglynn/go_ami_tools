package aws_bundle

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// According to the docs [1], different EC2 regions decrypt instance store AMIs
// using different private keys. The keys/IVs in the manifests must therefore
// be encrypted using different public keys.
//
// Keys are shipped with ec2-ami-tools [2] and do not appear to rotate, ever.
// This agrees with the manifest format, which still retains its 2007 version
// in 2016, and which has no apparent mechanism to specify which EC2
// certificate was used to encrypt the key + IV. It also agrees with the
// contents of cert-ec2.pem, which indicate that it expired in August 2006 but
// which remains in use as of August 2016.
//
// Certificates are included below.
//
//  [1] http://docs.aws.amazon.com/AWSEC2/latest/CommandLineReference/CLTRG-ami-bundle-image.html
//  [2] http://s3.amazonaws.com/ec2-downloads/ec2-ami-tools.zip

const certEc2 = `-----BEGIN CERTIFICATE-----
MIIDzjCCAzegAwIBAgIJALDnZV+lpZdSMA0GCSqGSIb3DQEBBQUAMIGhMQswCQYD
VQQGEwJaQTEVMBMGA1UECBMMV2VzdGVybiBDYXBlMRIwEAYDVQQHEwlDYXBlIFRv
d24xJzAlBgNVBAoTHkFtYXpvbiBEZXZlbG9wbWVudCBDZW50cmUgKFNBKTEMMAoG
A1UECxMDQUVTMREwDwYDVQQDEwhBRVMgVGVzdDEdMBsGCSqGSIb3DQEJARYOYWVz
QGFtYXpvbi5jb20wHhcNMDUwODA5MTYwMTA5WhcNMDYwODA5MTYwMTA5WjCBoTEL
MAkGA1UEBhMCWkExFTATBgNVBAgTDFdlc3Rlcm4gQ2FwZTESMBAGA1UEBxMJQ2Fw
ZSBUb3duMScwJQYDVQQKEx5BbWF6b24gRGV2ZWxvcG1lbnQgQ2VudHJlIChTQSkx
DDAKBgNVBAsTA0FFUzERMA8GA1UEAxMIQUVTIFRlc3QxHTAbBgkqhkiG9w0BCQEW
DmFlc0BhbWF6b24uY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC8v/X5
zZv8CAVfNmvBM0br/RUcf1wU8xC5d2otFQQsQKB3qiWoj3oHeOWskOlTPFVZ8N+/
hEaMjyOUkg2+g6XEagCQtFCEBzUVoMjiQIBPiWj5CWkFtlav2zt33LZ0ErTND4xl
j7FQFqbaytHU9xuQcFO2p12bdITiBs5Kwoi9bQIDAQABo4IBCjCCAQYwHQYDVR0O
BBYEFPQnsX1kDVzPtX+38ACV8RhoYcw8MIHWBgNVHSMEgc4wgcuAFPQnsX1kDVzP
tX+38ACV8RhoYcw8oYGnpIGkMIGhMQswCQYDVQQGEwJaQTEVMBMGA1UECBMMV2Vz
dGVybiBDYXBlMRIwEAYDVQQHEwlDYXBlIFRvd24xJzAlBgNVBAoTHkFtYXpvbiBE
ZXZlbG9wbWVudCBDZW50cmUgKFNBKTEMMAoGA1UECxMDQUVTMREwDwYDVQQDEwhB
RVMgVGVzdDEdMBsGCSqGSIb3DQEJARYOYWVzQGFtYXpvbi5jb22CCQCw52VfpaWX
UjAMBgNVHRMEBTADAQH/MA0GCSqGSIb3DQEBBQUAA4GBAJJlWll4uGlrqBzeIw7u
M3RvomlxMESwGKb9gI+ZeORlnHAyZxvd9XngIcjPuU+8uc3wc10LRQUCn45a5hFs
zaCp9BSewLCCirn6awZn2tP8JlagSbjrN9YShStt8S3S/Jj+eBoRvc7jJnmEeMkx
O0wHOzp5ZHRDK7tGULD6jCfU
-----END CERTIFICATE-----`

const certEc2Gov = `-----BEGIN CERTIFICATE-----
MIICvzCCAigCCQD3V6lFvX6dzDANBgkqhkiG9w0BAQUFADCBozELMAkGA1UEBhMC
VVMxCzAJBgNVBAgTAldBMRAwDgYDVQQHEwdTZWF0dGxlMRMwEQYDVQQKEwpBbWF6
b24uY29tMRYwFAYDVQQLEw1FQzIgQXV0aG9yaXR5MRowGAYDVQQDExFFQzIgQU1J
IEF1dGhvcml0eTEsMCoGCSqGSIb3DQEJARYdZWMyLWFtaS1nb3Ytd2VzdC0xQGFt
YXpvbi5jb20wHhcNMTEwODEyMTcyNjE1WhcNMjEwODA5MTcyNjE1WjCBozELMAkG
A1UEBhMCVVMxCzAJBgNVBAgTAldBMRAwDgYDVQQHEwdTZWF0dGxlMRMwEQYDVQQK
EwpBbWF6b24uY29tMRYwFAYDVQQLEw1FQzIgQXV0aG9yaXR5MRowGAYDVQQDExFF
QzIgQU1JIEF1dGhvcml0eTEsMCoGCSqGSIb3DQEJARYdZWMyLWFtaS1nb3Ytd2Vz
dC0xQGFtYXpvbi5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBANshKnhw
DUZ2/6VJwVTsXMUI1CGd5rpSpSLUCHGuqII+BDUvnp/sPxd1u6+I1QrbaaBAOm6+
evM77M7vNJXY3+JW00VOs9NgPEXBmn6UV4R1P7DljKurWGmRp8Fj1yVU4sSgZqqv
74SyhD0Z4ASczVcOiTZICeuQoJwmeZ8F20oLAgMBAAEwDQYJKoZIhvcNAQEFBQAD
gYEAH3vpkD80ngP1e18UYSVBCODArik+aeUPAzJrPDYorrnffbamks50IMTktyiu
za1JuplrvVsAKcQhyoPOq69bwRDg4L8VOXSCjjvsNuEhHL603h8jn6ghEouPCPl7
8s4Sr5XikmAgwFcPb/frNnLuZsSol08tISgPOlFg4KLv/bo=
-----END CERTIFICATE-----`

const certEc2CnNorth1 = `-----BEGIN CERTIFICATE-----
MIIEwTCCA6mgAwIBAgIJALBg5STuwebSMA0GCSqGSIb3DQEBBQUAMIGbMQswCQYD
VQQGEwJaQTEVMBMGA1UECBMMV2VzdGVybiBDYXBlMRIwEAYDVQQHEwlDYXBlIFRv
d24xHDAaBgNVBAoTE0FtYXpvbiBXZWIgU2VydmljZXMxDDAKBgNVBAsTA0VDMjEW
MBQGA1UEAxMNQU1JIE1hbmlmZXN0czEdMBsGCSqGSIb3DQEJARYOYWVzQGFtYXpv
bi5jb20wHhcNMTMwODI4MTAxNTEwWhcNMTkwMjE4MTAxNTEwWjCBmzELMAkGA1UE
BhMCWkExFTATBgNVBAgTDFdlc3Rlcm4gQ2FwZTESMBAGA1UEBxMJQ2FwZSBUb3du
MRwwGgYDVQQKExNBbWF6b24gV2ViIFNlcnZpY2VzMQwwCgYDVQQLEwNFQzIxFjAU
BgNVBAMTDUFNSSBNYW5pZmVzdHMxHTAbBgkqhkiG9w0BCQEWDmFlc0BhbWF6b24u
Y29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA601VE9anUnkM5weg
THEUqNkT7fwaKKoTM+oV6n0h5D4LF/qY7IuS7YlxP308P00yE6In4ZHHvc1E/X1e
s36t7ojdlx8Pg0jzFLir27BWSl+ddZGJLl0u8FFzJUPmqeiQZW3mKosOqAoj/C5h
svXnSmLGxd619eGz2+Kel38LPhwzA8PZeNhil/eK53oo/h7dZvRUtH2+zbYXxq5Q
2tsRN1Ef4QaNeKHSdupo8eFNUnmSgeEYz1RC/2QDn6MK7uXijOft4G6iEO4c+PeR
wLyBW8YMla2SJk1n4z4gmSJ21uCZ0Bv0A2j/3f+JmgNo4QLIAl/UBWBTT+eSAFYw
LVDlrwIDAQABo4IBBDCCAQAwHQYDVR0OBBYEFJSvQRrRg2O1O/kSOEKH2z18OFfH
MIHQBgNVHSMEgcgwgcWAFJSvQRrRg2O1O/kSOEKH2z18OFfHoYGhpIGeMIGbMQsw
CQYDVQQGEwJaQTEVMBMGA1UECBMMV2VzdGVybiBDYXBlMRIwEAYDVQQHEwlDYXBl
IFRvd24xHDAaBgNVBAoTE0FtYXpvbiBXZWIgU2VydmljZXMxDDAKBgNVBAsTA0VD
MjEWMBQGA1UEAxMNQU1JIE1hbmlmZXN0czEdMBsGCSqGSIb3DQEJARYOYWVzQGFt
YXpvbi5jb22CCQCwYOUk7sHm0jAMBgNVHRMEBTADAQH/MA0GCSqGSIb3DQEBBQUA
A4IBAQACBMJpb8N7cT0PP3u814D1Ngd2vqEv6aB8saklT44kWwAXDcILVtPd09ae
8q1oWSKpWlGo9Z8gUS92QXMIMxSZCxDdN4MflYWGio5HFvpS/msHVkK9H80nypSd
pLS3FP0arr/3tETS8TIhs4aISwUUfHm0W7WrmLaQz8TyfuktVtPrKIMmWgXiJmCo
HQkuFe4rjx0y7r8CGQocwo79+m+35aLip44jWB4yLuUgp0wVhT5nxfG/iNX2lUiP
Bw/yCpzeJoLBWvFDlunBNu2s0Y3ddFdnlna/k7CQM1Js6+OGQBMh1zTtJlPkkHj3
mbaTR6i5yro01FowChTryrRTVfMe
-----END CERTIFICATE-----`

func CertificateForEC2Region(region string) (*x509.Certificate, error) {
	var pemStr string

	// Docs:
	//  --ec2cert path
	//    The path to the Amazon EC2 X.509 public key certificate used to encrypt the image manifest.
	//    Required: Only for the us-gov-west-1 and cn-north-1 regions.
	// http://docs.aws.amazon.com/AWSEC2/latest/CommandLineReference/CLTRG-ami-bundle-image.html

	switch region {
	case "us-gov-west-1":
		pemStr = certEc2Gov
	case "cn-north-1":
		pemStr = certEc2CnNorth1
	default:
		pemStr = certEc2
	}

	// Parse the PEM block to get DER
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("unable to parse PEM block")
	}

	// Parse the DER to get a certificate
	return x509.ParseCertificate(block.Bytes)
}
