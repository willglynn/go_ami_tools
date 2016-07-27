package aws_bundle

import (
	"crypto/rsa"
	"testing"
)

func TestCertificateForEC2Region(t *testing.T) {
	// produced by `openssl x509`ing the official PEMs
	const modulusCertEc2 = "bcbff5f9cd9bfc08055f366bc13346ebfd151c7f5c14f310b9776a2d15042c40a077aa25a88f7a0778e5ac90e9533c5559f0dfbf84468c8f2394920dbe83a5c46a0090b45084073515a0c8e240804f8968f9096905b656afdb3b77dcb67412b4cd0f8c658fb15016a6dacad1d4f71b907053b6a75d9b7484e206ce4ac288bd6d"
	const modulusCertEc2Gov = "db212a78700d4676ffa549c154ec5cc508d4219de6ba52a522d40871aea8823e04352f9e9fec3f1775bbaf88d50adb69a0403a6ebe7af33becceef3495d8dfe256d3454eb3d3603c45c19a7e945784753fb0e58cabab586991a7c163d72554e2c4a066aaafef84b2843d19e0049ccd570e89364809eb90a09c26799f05db4a0b"
	const modulusCertEc2CnNorth1 = "eb4d5513d6a752790ce707a04c7114a8d913edfc1a28aa1333ea15ea7d21e43e0b17fa98ec8b92ed89713f7d3c3f4d3213a227e191c7bdcd44fd7d5eb37eadee88dd971f0f8348f314b8abdbb0564a5f9d7591892e5d2ef051732543e6a9e890656de62a8b0ea80a23fc2e61b2f5e74a62c6c5deb5f5e1b3dbe29e977f0b3e1c3303c3d978d86297f78ae77a28fe1edd66f454b47dbecdb617c6ae50dadb1137511fe1068d78a1d276ea68f1e14d52799281e118cf5442ff64039fa30aeee5e28ce7ede06ea210ee1cf8f791c0bc815bc60c95ad92264d67e33e20992276d6e099d01bf40368ffddff899a0368e102c8025fd40560534fe7920056302d50e5af"

	tests := []struct {
		region   string
		expected string
	}{
		{"us-east-1", modulusCertEc2},
		{"us-west-2", modulusCertEc2},
		{"us-future-region-47", modulusCertEc2},
		{"atlantis-4", modulusCertEc2},
		{"us-gov-west-1", modulusCertEc2Gov},
		{"cn-north-1", modulusCertEc2CnNorth1},
	}
	for _, tt := range tests {
		cert, err := CertificateForEC2Region(tt.region)
		if err != nil {
			t.Errorf("CertificateForEC2Region(%q) error = %v", tt.region, err)
			continue
		}

		var modulus string
		if cert != nil && cert.PublicKey != nil {
			modulus = cert.PublicKey.(*rsa.PublicKey).N.Text(16)
		}

		if modulus != tt.expected {
			t.Errorf("CertificateForEC2Region(%q) = modulus %v, expected %v", tt.region, modulus, tt.expected)
		}
	}
}
