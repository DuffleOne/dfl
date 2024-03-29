package app

import (
	"dfl/tools/certgen"
)

// GenerateClientCertificate generates a new client certificate
func (a *App) GenerateClientCertificate(name, password string) error {
	err := a.checkForInit()
	if err != nil {
		return err
	}

	nextSerial, err := a.getNextSerial(certgen.ClientCertificate, name)
	if err != nil {
		return err
	}

	rootCA, err := a.getCertificate(certgen.RootCA, "root")
	if err != nil {
		return err
	}

	rootPrivate, err := a.loadPrivate(certgen.RootCA, "root")
	if err != nil {
		return err
	}

	private, err := a.getOrSetPrivate(certgen.ClientCertificate, name)
	if err != nil {
		return err
	}

	certTemplate, err := a.createTemplate(certgen.ClientCertificate, nextSerial)
	if err != nil {
		return err
	}

	certTemplate.Subject.CommonName = name

	certificate, err := a.createCertificate(&certSignReq{
		certType:          certgen.ClientCertificate,
		template:          certTemplate,
		parent:            rootCA,
		certificatePublic: private.Public(),
		parentPrivate:     rootPrivate,
	})
	if err != nil {
		return err
	}

	return a.saveP12(certgen.ClientCertificate, &saveP12Req{
		name:        name,
		private:     private,
		certificate: certificate,
		rootCA:      rootCA,
		password:    password,
	})
}
