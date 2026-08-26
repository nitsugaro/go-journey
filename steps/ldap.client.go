package steps

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/nitsugaro/go-journey/types"
)

const LDAPClientCacheKey = "ldap_client"

type LDAPRepository interface {
	Search(context.Context, *LDAPSearchRequest) (*LDAPSearchResult, error)
	Bind(context.Context, string, string) error
	Compare(context.Context, string, string, string) (bool, error)
	Modify(context.Context, *LDAPModifyRequest) error
	Add(context.Context, *LDAPAddRequest) error
	Delete(context.Context, string) error
	ModifyDN(context.Context, *LDAPModifyDNRequest) error
}

type LDAPClientConfig struct {
	URLs                    []string       `json:"urls,omitempty" description:"LDAP server URLs. Example: ldap://127.0.0.1:389 or ldaps://ldap.example.com:636."`
	Host                    string         `json:"host,omitempty" description:"LDAP server host used when urls is empty."`
	Port                    int            `json:"port,omitempty" minimum:"0" description:"LDAP server port used with host. Defaults to 389 or 636 when TLS is enabled."`
	BaseDN                  string         `json:"base_dn,omitempty" description:"Default base DN used by LDAP operations when the step does not override it."`
	UseTLS                  bool           `json:"use_tls,omitempty" description:"Connect using LDAPS from the beginning."`
	StartTLS                bool           `json:"start_tls,omitempty" description:"Upgrade a plain LDAP connection using StartTLS after connecting."`
	ServerName              string         `json:"server_name,omitempty" description:"TLS server name used for certificate validation."`
	InsecureSkipVerify      bool           `json:"insecure_skip_verify,omitempty" description:"Skip TLS server certificate verification. Use only in controlled environments."`
	RootCAs                 []string       `json:"root_cas,omitempty" description:"PEM values or file paths for trusted LDAP CA certificates."`
	ClientCert              string         `json:"client_cert,omitempty" description:"PEM value or file path for the client certificate used by mutual TLS."`
	ClientKey               string         `json:"client_key,omitempty" description:"PEM value or file path for the client private key used by mutual TLS."`
	Bind                    LDAPBindConfig `json:"bind,omitempty" description:"Optional service bind used when pooled connections are created."`
	ConnectTimeoutSeconds   int            `json:"connect_timeout_seconds,omitempty" minimum:"0" description:"Connection timeout in seconds. Empty or 0 uses the LDAP client default."`
	OperationTimeoutSeconds int            `json:"operation_timeout_seconds,omitempty" minimum:"0" description:"Operation timeout in seconds. Empty or 0 uses the LDAP client default."`
	Pool                    LDAPPoolConfig `json:"pool,omitempty" description:"Connection pool limits for this LDAP instance."`
}

type LDAPBindConfig struct {
	Method        string            `json:"method,omitempty" enum:"simple,unauthenticated,external,ntlm_unauthenticated,digest_md5" default:"simple" description:"Bind method used by pooled connections."`
	DN            string            `json:"dn,omitempty" description:"Bind DN for simple or unauthenticated bind."`
	Password      string            `json:"password,omitempty" description:"Bind password. Supports config placeholders."`
	Username      string            `json:"username,omitempty" description:"Username used by SASL/NTLM bind methods."`
	Realm         string            `json:"realm,omitempty" description:"Realm or domain used by bind methods that require it."`
	SASLMechanism string            `json:"sasl_mechanism,omitempty" description:"Optional SASL mechanism name for providers that need it."`
	Properties    map[string]string `json:"properties,omitempty" additionalProperties.type:"string" description:"Provider-specific bind properties."`
}

type LDAPPoolConfig struct {
	MaxOpen            int `json:"max_open,omitempty" minimum:"0" default:"10" description:"Maximum open LDAP connections for this instance."`
	MaxIdle            int `json:"max_idle,omitempty" minimum:"0" description:"Maximum idle LDAP connections. Empty or 0 uses max_open."`
	MaxLifetimeSeconds int `json:"max_lifetime_seconds,omitempty" minimum:"0" description:"Maximum connection lifetime in seconds. Empty or 0 disables lifetime recycling."`
}

type LDAPSearchRequest struct {
	BaseDN           string
	Scope            string
	DerefAliases     string
	Filter           string
	Attributes       []string
	SizeLimit        int
	TimeLimitSeconds int
	TypesOnly        bool
}

type LDAPSearchResult struct {
	Entries []LDAPEntry
}

type LDAPEntry struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
}

type LDAPModification struct {
	Operation string   `json:"operation" enum:"add,delete,replace,increment" default:"replace"`
	Attribute string   `json:"attribute"`
	Values    []string `json:"values,omitempty"`
}

type LDAPModifyRequest struct {
	DN      string
	Changes []LDAPModification
}

type LDAPAddRequest struct {
	DN         string
	Attributes map[string][]string
}

type LDAPModifyDNRequest struct {
	DN           string
	RDN          string
	DeleteOldRDN bool
	NewSuperior  string
}

type ldapPooledConn struct {
	conn      *ldap.Conn
	createdAt time.Time
}

type LDAPClientPool struct {
	config    LDAPClientConfig
	urls      []string
	tlsConfig *tls.Config
	timeout   time.Duration
	lifetime  time.Duration
	maxOpen   int
	maxIdle   int

	mu     sync.Mutex
	idle   chan *ldapPooledConn
	open   int
	nextID int
}

func LDAPClientFactory(config json.RawMessage) (any, error) {
	var clientConfig LDAPClientConfig
	if err := json.Unmarshal(config, &clientConfig); err != nil {
		return nil, err
	}
	return NewLDAPClientPool(&clientConfig)
}

func NewLDAPClientPool(config *LDAPClientConfig) (*LDAPClientPool, error) {
	if config == nil {
		return nil, errors.New("LDAP client config is nil")
	}
	urls := ldapURLs(config)
	if len(urls) == 0 {
		return nil, errors.New("LDAP urls or host are required")
	}
	tlsConfig, err := ldapTLSConfig(config)
	if err != nil {
		return nil, err
	}
	maxOpen := config.Pool.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 10
	}
	maxIdle := config.Pool.MaxIdle
	if maxIdle < 0 {
		return nil, errors.New("LDAP max_idle cannot be negative")
	}
	if maxIdle == 0 || maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	timeout := time.Duration(config.OperationTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &LDAPClientPool{
		config: *config, urls: urls, tlsConfig: tlsConfig, timeout: timeout,
		lifetime: time.Duration(config.Pool.MaxLifetimeSeconds) * time.Second,
		maxOpen:  maxOpen, maxIdle: maxIdle, idle: make(chan *ldapPooledConn, maxIdle),
	}, nil
}

func (pool *LDAPClientPool) Search(ctx context.Context, request *LDAPSearchRequest) (*LDAPSearchResult, error) {
	if request == nil {
		return nil, errors.New("LDAP search request is nil")
	}
	conn, err := pool.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.release(conn)
	baseDN := request.BaseDN
	if baseDN == "" {
		baseDN = pool.config.BaseDN
	}
	result, err := conn.conn.Search(ldap.NewSearchRequest(
		baseDN,
		ldapScope(request.Scope),
		ldapDerefAliases(request.DerefAliases),
		request.SizeLimit,
		request.TimeLimitSeconds,
		request.TypesOnly,
		request.Filter,
		request.Attributes,
		nil,
	))
	if err != nil {
		return nil, err
	}
	entries := make([]LDAPEntry, 0, len(result.Entries))
	for _, entry := range result.Entries {
		attributes := map[string][]string{}
		for _, attribute := range entry.Attributes {
			attributes[attribute.Name] = append([]string(nil), attribute.Values...)
		}
		entries = append(entries, LDAPEntry{DN: entry.DN, Attributes: attributes})
	}
	return &LDAPSearchResult{Entries: entries}, nil
}

func (pool *LDAPClientPool) Bind(ctx context.Context, dn, password string) error {
	conn, err := pool.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if password == "" {
		return conn.UnauthenticatedBind(dn)
	}
	return conn.Bind(dn, password)
}

func (pool *LDAPClientPool) Compare(ctx context.Context, dn, attribute, value string) (bool, error) {
	conn, err := pool.acquire(ctx)
	if err != nil {
		return false, err
	}
	defer pool.release(conn)
	return conn.conn.Compare(dn, attribute, value)
}

func (pool *LDAPClientPool) Modify(ctx context.Context, request *LDAPModifyRequest) error {
	if request == nil {
		return errors.New("LDAP modify request is nil")
	}
	conn, err := pool.acquire(ctx)
	if err != nil {
		return err
	}
	defer pool.release(conn)
	modify := ldap.NewModifyRequest(request.DN, nil)
	for _, change := range request.Changes {
		switch strings.ToLower(change.Operation) {
		case "add":
			modify.Add(change.Attribute, change.Values)
		case "delete":
			modify.Delete(change.Attribute, change.Values)
		case "replace":
			modify.Replace(change.Attribute, change.Values)
		case "increment":
			if len(change.Values) != 1 {
				return errors.New("LDAP increment requires exactly one value")
			}
			modify.Increment(change.Attribute, change.Values[0])
		default:
			return fmt.Errorf("unsupported LDAP modify operation %q", change.Operation)
		}
	}
	return conn.conn.Modify(modify)
}

func (pool *LDAPClientPool) Add(ctx context.Context, request *LDAPAddRequest) error {
	if request == nil {
		return errors.New("LDAP add request is nil")
	}
	conn, err := pool.acquire(ctx)
	if err != nil {
		return err
	}
	defer pool.release(conn)
	add := ldap.NewAddRequest(request.DN, nil)
	for attribute, values := range request.Attributes {
		add.Attribute(attribute, values)
	}
	return conn.conn.Add(add)
}

func (pool *LDAPClientPool) Delete(ctx context.Context, dn string) error {
	conn, err := pool.acquire(ctx)
	if err != nil {
		return err
	}
	defer pool.release(conn)
	return conn.conn.Del(ldap.NewDelRequest(dn, nil))
}

func (pool *LDAPClientPool) ModifyDN(ctx context.Context, request *LDAPModifyDNRequest) error {
	if request == nil {
		return errors.New("LDAP modify DN request is nil")
	}
	conn, err := pool.acquire(ctx)
	if err != nil {
		return err
	}
	defer pool.release(conn)
	return conn.conn.ModifyDN(ldap.NewModifyDNRequest(request.DN, request.RDN, request.DeleteOldRDN, request.NewSuperior))
}

func (pool *LDAPClientPool) acquire(ctx context.Context) (*ldapPooledConn, error) {
	for {
		select {
		case conn := <-pool.idle:
			if pool.expired(conn) {
				pool.discard(conn)
				continue
			}
			return conn, nil
		default:
		}
		pool.mu.Lock()
		if pool.open < pool.maxOpen {
			pool.open++
			pool.mu.Unlock()
			conn, err := pool.dial(ctx)
			if err != nil {
				pool.mu.Lock()
				pool.open--
				pool.mu.Unlock()
				return nil, err
			}
			if err := pool.configuredBind(conn); err != nil {
				conn.Close()
				pool.mu.Lock()
				pool.open--
				pool.mu.Unlock()
				return nil, err
			}
			return &ldapPooledConn{conn: conn, createdAt: time.Now()}, nil
		}
		pool.mu.Unlock()
		select {
		case conn := <-pool.idle:
			if pool.expired(conn) {
				pool.discard(conn)
				continue
			}
			return conn, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (pool *LDAPClientPool) release(conn *ldapPooledConn) {
	if conn == nil || pool.expired(conn) {
		pool.discard(conn)
		return
	}
	select {
	case pool.idle <- conn:
	default:
		pool.discard(conn)
	}
}

func (pool *LDAPClientPool) discard(conn *ldapPooledConn) {
	if conn != nil && conn.conn != nil {
		conn.conn.Close()
	}
	pool.mu.Lock()
	if pool.open > 0 {
		pool.open--
	}
	pool.mu.Unlock()
}

func (pool *LDAPClientPool) expired(conn *ldapPooledConn) bool {
	return pool.lifetime > 0 && time.Since(conn.createdAt) >= pool.lifetime
}

func (pool *LDAPClientPool) dial(ctx context.Context) (*ldap.Conn, error) {
	timeout := time.Duration(pool.config.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	options := []ldap.DialOpt{ldap.DialWithDialer(dialer)}
	if pool.tlsConfig != nil {
		options = append(options, ldap.DialWithTLSConfig(pool.tlsConfig))
	}
	var lastErr error
	for attempt := 0; attempt < len(pool.urls); attempt++ {
		url := pool.urls[pool.nextURL()]
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		conn, err := ldap.DialURL(url, options...)
		if err != nil {
			lastErr = err
			continue
		}
		conn.SetTimeout(pool.timeout)
		if pool.config.StartTLS {
			if err := conn.StartTLS(pool.tlsConfig); err != nil {
				conn.Close()
				lastErr = err
				continue
			}
		}
		return conn, nil
	}
	return nil, lastErr
}

func (pool *LDAPClientPool) nextURL() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	id := pool.nextID
	pool.nextID++
	return id % len(pool.urls)
}

func (pool *LDAPClientPool) configuredBind(conn *ldap.Conn) error {
	method := strings.ToLower(pool.config.Bind.Method)
	if method == "" {
		method = "none"
	}
	switch method {
	case "none", "anonymous":
		return nil
	case "simple":
		return conn.Bind(pool.config.Bind.DN, pool.config.Bind.Password)
	case "unauthenticated":
		return conn.UnauthenticatedBind(pool.config.Bind.DN)
	case "external", "sasl_external":
		return conn.ExternalBind()
	case "ntlm":
		return conn.NTLMBind(pool.config.Bind.Realm, pool.config.Bind.Username, pool.config.Bind.Password)
	case "ntlm_unauthenticated":
		return conn.NTLMUnauthenticatedBind(pool.config.Bind.Realm, pool.config.Bind.Username)
	case "digest_md5", "sasl_digest_md5":
		_, err := conn.DigestMD5Bind(&ldap.DigestMD5BindRequest{
			Host:     pool.config.Bind.Properties["host"],
			Username: pool.config.Bind.Username,
			Password: pool.config.Bind.Password,
		})
		return err
	default:
		return fmt.Errorf("unsupported LDAP bind method %q", pool.config.Bind.Method)
	}
}

func ldapURLs(config *LDAPClientConfig) []string {
	if config == nil {
		return nil
	}
	if len(config.URLs) > 0 {
		return append([]string(nil), config.URLs...)
	}
	if config.Host == "" {
		return nil
	}
	port := config.Port
	if port == 0 {
		if config.UseTLS {
			port = 636
		} else {
			port = 389
		}
	}
	scheme := "ldap"
	if config.UseTLS {
		scheme = "ldaps"
	}
	return []string{fmt.Sprintf("%s://%s:%d", scheme, config.Host, port)}
}

func ldapTLSConfig(config *LDAPClientConfig) (*tls.Config, error) {
	if config == nil {
		return nil, errors.New("LDAP client config is nil")
	}
	tlsConfig := &tls.Config{ServerName: config.ServerName, InsecureSkipVerify: config.InsecureSkipVerify} //nolint:gosec
	if len(config.RootCAs) > 0 {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		for _, source := range config.RootCAs {
			pem, err := ldapPEM(source)
			if err != nil {
				return nil, err
			}
			if !pool.AppendCertsFromPEM(pem) {
				return nil, errors.New("invalid LDAP root CA")
			}
		}
		tlsConfig.RootCAs = pool
	}
	if config.ClientCert != "" || config.ClientKey != "" {
		certPEM, err := ldapPEM(config.ClientCert)
		if err != nil {
			return nil, err
		}
		keyPEM, err := ldapPEM(config.ClientKey)
		if err != nil {
			return nil, err
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

func ldapPEM(value string) ([]byte, error) {
	if strings.Contains(value, "-----BEGIN") {
		return []byte(value), nil
	}
	return os.ReadFile(value)
}

func ldapScope(value string) int {
	switch strings.ToLower(value) {
	case "base":
		return ldap.ScopeBaseObject
	case "one":
		return ldap.ScopeSingleLevel
	default:
		return ldap.ScopeWholeSubtree
	}
}

func ldapDerefAliases(value string) int {
	switch strings.ToLower(value) {
	case "searching":
		return ldap.DerefInSearching
	case "finding":
		return ldap.DerefFindingBaseObj
	case "always":
		return ldap.DerefAlways
	default:
		return ldap.NeverDerefAliases
	}
}

func transactionLDAPRepository(transaction *types.JourneyTransaction, connection string) (LDAPRepository, error) {
	if transaction.CacheManager == nil {
		return nil, errors.New("LDAP cache manager is not configured")
	}
	instance, ok := transaction.CacheManager.GetCacheInstance(LDAPClientCacheKey, connection)
	if !ok {
		return nil, fmt.Errorf("LDAP connection %q not found", connection)
	}
	repository, ok := instance.(LDAPRepository)
	if !ok {
		return nil, fmt.Errorf("LDAP connection %q has invalid type", connection)
	}
	return repository, nil
}

func transactionContext(transaction *types.JourneyTransaction) context.Context {
	if transaction.Context != nil {
		return transaction.Context
	}
	return context.Background()
}

func setLDAPOutput(transaction *types.JourneyTransaction, output string, value any) error {
	ctx, key := transaction.State.GetCtxPath(output)
	if ctx == nil || key == "" {
		return types.StepInvalidConfig(transaction.CurrentStepID, "invalid LDAP output context path")
	}
	ctx.Set(key, value)
	return nil
}

func ldapWriteOutcome(err error) string {
	if err == nil {
		return "success"
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
		return "not_found"
	}
	return "error"
}
