package x509

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/cloudbase/garm-provider-common/util"
)

func RawCABundleToMap(bundle []byte) (map[string][]byte, error) {
	if len(bundle) == 0 {
		// passing in an empty byte array is not an error case. We can just
		// return an empty map.
		return make(map[string][]byte), nil
	}
	ret := map[string][]byte{}

	var block *pem.Block
	rest := bundle
	for {
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		pub, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		out := &bytes.Buffer{}
		if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: block.Bytes}); err != nil {
			return nil, err
		}
		ret[fmt.Sprintf("%d", pub.SerialNumber)] = out.Bytes()
	}

	return ret, nil
}

func CombineAndDeduplicateBundles(bundles ...[]byte) ([]byte, error) {
	if len(bundles) == 0 {
		return []byte{}, nil
	}
	out := &bytes.Buffer{}
	for _, val := range bundles {
		val = bytes.Trim(val, "\r\n")
		if len(val) == 0 {
			continue
		}
		out.Write(val)
		out.Write([]byte("\n"))
	}

	data := out.Bytes()
	if len(data) == 0 {
		return []byte{}, nil
	}

	sanitized, err := util.SanitizeCABundle(data)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize bundles: %w", err)
	}
	return sanitized, nil
}
