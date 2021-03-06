//支持连接https+grpc共享端口的版本，也支持非证书版
//Usage:
//conn, err := client.NewClientConn("localhost:63333", "server.crt", "server.grpc.io",
// grpc.WithPerRPCCredentials(&rpc.X{Value: "abc", Key: "x"}))
//client := rpc.NewHelloServiceClient(conn)
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"

	"github.com/hsyan2008/hfw2/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ClientCreds struct {
	ServerName string
	CaFile     string
	CertFile   string
	KeyFile    string
}

func NewClientConn(ctx context.Context, address string, opt ...grpc.DialOption) (*grpc.ClientConn, error) {
	if len(address) == 0 {
		return nil, errors.New("nil address")
	}

	opt = append(opt, grpc.WithInsecure())

	return grpc.DialContext(ctx, address, opt...)
}

func NewSecurityClientConn(ctx context.Context, address, certFile, serverName string, opt ...grpc.DialOption) (*grpc.ClientConn, error) {
	if len(address) == 0 || len(serverName) == 0 {
		return nil, errors.New("nil address or serverName")
	}

	if common.IsExist(certFile) {
		t := &ClientCreds{
			CertFile:   certFile,
			ServerName: serverName,
		}

		creds, err := t.GetCredentials()
		if err != nil {
			return nil, err
		}

		opt = append(opt, grpc.WithTransportCredentials(creds))
	}

	return grpc.DialContext(ctx, address, opt...)
}

func (t *ClientCreds) GetCredentials() (credentials.TransportCredentials, error) {
	if len(t.CaFile) > 0 {
		return t.GetCredentialsByCA()
	}

	return t.GetTLSCredentials()
}

func (t *ClientCreds) GetCredentialsByCA() (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(t.CaFile)
	if err != nil {
		return nil, err
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, err
	}

	c := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   t.ServerName,
		RootCAs:      certPool,
	})

	return c, err
}

func (t *ClientCreds) GetTLSCredentials() (credentials.TransportCredentials, error) {
	c, err := credentials.NewClientTLSFromFile(t.CertFile, t.ServerName)
	if err != nil {
		return nil, err
	}

	return c, err
}
