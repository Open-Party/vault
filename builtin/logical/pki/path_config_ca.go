package pki

import (
	"fmt"

	"github.com/hashicorp/vault/helper/certutil"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

func pathConfigCA(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/ca",
		Fields: map[string]*framework.FieldSchema{
			"pem_bundle": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "PEM-format, concatenated unencrypted secret key and certificate",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.WriteOperation: b.pathCAWrite,
		},

		HelpSynopsis:    pathConfigCAHelpSyn,
		HelpDescription: pathConfigCAHelpDesc,
	}
}

func (b *backend) pathCAWrite(
	req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	pemBundle := d.Get("pem_bundle").(string)

	parsedBundle, err := certutil.ParsePEMBundle(pemBundle)
	if err != nil {
		switch err.(type) {
		case certutil.InternalError:
			return nil, err
		default:
			return logical.ErrorResponse(err.Error()), nil
		}
	}

	// TODO?: CRLs can only be generated with RSA keys right now, in the
	// Go standard library. The plubming is here to support non-RSA keys
	// if the library gets support

	if parsedBundle.PrivateKeyType != certutil.RSAPrivateKey {
		return logical.ErrorResponse("Currently, only RSA keys are supported for the CA certificate"), nil
	}

	if !parsedBundle.Certificate.IsCA {
		return logical.ErrorResponse("The given certificate is not marked for CA use and cannot be used with this backend"), nil
	}

	cb, err := parsedBundle.ToCertBundle()
	if err != nil {
		return nil, fmt.Errorf("Error converting raw values into cert bundle: %s", err)
	}

	entry, err := logical.StorageEntryJSON("config/ca_bundle", cb)
	if err != nil {
		return nil, err
	}
	err = req.Storage.Put(entry)
	if err != nil {
		return nil, err
	}

	// For ease of later use, also store just the certificate at a known
	// location, plus a blank CRL
	entry.Key = "ca"
	entry.Value = parsedBundle.CertificateBytes
	err = req.Storage.Put(entry)
	if err != nil {
		return nil, err
	}

	entry.Key = "crl"
	entry.Value = []byte{}
	err = req.Storage.Put(entry)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

const pathConfigCAHelpSyn = `
Configure the CA certificate and private key used for generated credentials.
`

const pathConfigCAHelpDesc = `
This configures the CA information used for credentials
generated by this backend. This must be a PEM-format, concatenated
unencrypted secret key and certificate.

For security reasons, you can only view the certificate when reading this endpoint.
`
