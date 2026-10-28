package x509certpoolfixture

import (
	"crypto/x509"
	"os"
)

type FakePool struct{}

type Provider interface {
	CertPool() *x509.CertPool
}

type CertPoolAdder interface {
	AddTo(*x509.CertPool, *x509.Certificate)
}

type ExplicitAdder struct{}

func (ExplicitAdder) AddTo(pool *x509.CertPool, cert *x509.Certificate) {
	pool.AddCert(cert)
}

func (*FakePool) AddCert(*x509.Certificate)                                                {}
func (*FakePool) AppendCertsFromPEM([]byte) bool                                           { return false }
func (*FakePool) AddCertWithConstraint(*x509.Certificate, func([]*x509.Certificate) error) {}
func (*FakePool) NewCertPool() *x509.CertPool                                              { return nil }
func (*FakePool) SystemCertPool() (*x509.CertPool, error)                                  { return nil, nil }

func NewPool() *x509.CertPool {
	return x509.NewCertPool()
}

func PoolOperations(pool *x509.CertPool, cert *x509.Certificate, pemBytes []byte) {
	pool.AddCert(cert)
	_ = pool.AppendCertsFromPEM(pemBytes)
	pool.AddCertWithConstraint(cert, func([]*x509.Certificate) error {
		_ = os.WriteFile("constraint-callback", nil, 0o600)
		return nil
	})
}

func UseAdder(adder CertPoolAdder, pool *x509.CertPool, cert *x509.Certificate) {
	adder.AddTo(pool, cert)
}

func CandidatePath(pool *x509.CertPool, cert *x509.Certificate) {
	UseAdder(ExplicitAdder{}, pool, cert)
}

func Helper(pool *x509.CertPool, cert *x509.Certificate) {
	pool.AddCert(cert)
}

func Caller(pool *x509.CertPool, cert *x509.Certificate) {
	Helper(pool, cert)
}

func SameNamedMethods(pool *FakePool, cert *x509.Certificate, pemBytes []byte) {
	pool.AddCert(cert)
	_ = pool.AppendCertsFromPEM(pemBytes)
	pool.AddCertWithConstraint(cert, nil)
	_ = pool.NewCertPool()
	_, _ = pool.SystemCertPool()
}

func SystemRoots() {
	_, _ = x509.SystemCertPool()
}

func UseProvider(provider Provider) {
	_ = provider.CertPool()
}
