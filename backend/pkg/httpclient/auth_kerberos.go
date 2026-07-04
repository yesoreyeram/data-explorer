package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jcmturner/gokrb5/v8/spnego"
)

// KerberosConfig configures Kerberos V5 / SPNEGO authentication (the
// `Authorization: Negotiate <token>` scheme used by many enterprise
// databases, internal APIs, and Windows-integrated services). Backed by
// github.com/jcmturner/gokrb5, a pure-Go krb5 implementation - no cgo or
// system MIT/Heimdal Kerberos libraries required.
type KerberosConfig struct {
	// KRB5ConfPath is the path to a krb5.conf describing the realm's KDCs.
	KRB5ConfPath string
	Realm        string
	Username     string

	// Exactly one of Password or KeytabPath must be set.
	Password   string
	KeytabPath string

	// SPN is the target service principal name, e.g. "HTTP/db.internal.example.com".
	SPN string
}

// KerberosAuth authenticates using SPNEGO, obtaining and caching a Kerberos
// client (and the service tickets it manages internally) on first use.
type KerberosAuth struct {
	cfg KerberosConfig

	mu  sync.Mutex
	cl  *client.Client
	err error
}

func NewKerberosAuth(cfg KerberosConfig) *KerberosAuth {
	return &KerberosAuth{cfg: cfg}
}

func (a *KerberosAuth) Authenticate(_ context.Context, req *http.Request) error {
	cl, err := a.client()
	if err != nil {
		return fmt.Errorf("httpclient: kerberos: %w", err)
	}
	if err := spnego.SetSPNEGOHeader(cl, req, a.cfg.SPN); err != nil {
		return fmt.Errorf("httpclient: kerberos: set SPNEGO header: %w", err)
	}
	return nil
}

func (a *KerberosAuth) client() (*client.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cl != nil {
		return a.cl, nil
	}
	if a.err != nil {
		return nil, a.err
	}

	krb5conf, err := config.Load(a.cfg.KRB5ConfPath)
	if err != nil {
		a.err = fmt.Errorf("load krb5.conf: %w", err)
		return nil, a.err
	}

	var cl *client.Client
	switch {
	case a.cfg.KeytabPath != "":
		kt, ktErr := keytab.Load(a.cfg.KeytabPath)
		if ktErr != nil {
			a.err = fmt.Errorf("load keytab: %w", ktErr)
			return nil, a.err
		}
		cl = client.NewWithKeytab(a.cfg.Username, a.cfg.Realm, kt, krb5conf)
	case a.cfg.Password != "":
		cl = client.NewWithPassword(a.cfg.Username, a.cfg.Realm, a.cfg.Password, krb5conf)
	default:
		a.err = fmt.Errorf("either Password or KeytabPath must be set")
		return nil, a.err
	}

	if loginErr := cl.Login(); loginErr != nil {
		a.err = fmt.Errorf("kerberos login: %w", loginErr)
		return nil, a.err
	}

	a.cl = cl
	return a.cl, nil
}
