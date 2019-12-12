package subnet_manger_client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Mellanox/ib-sriov-cni/pkg/types"
	"io"
	"net/http"
	"strings"
)

type ufmConnectionDetails struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	HttpSchema string `json:"httpSchema"`
}

type ufmSubnetMangerClient struct {
	connectionDetails *ufmConnectionDetails
	client            http.Client
}

func newUfmSubnetMangerClient(connectionDetails []byte) (types.SubnetMangerClient, error) {
	connDetails := &ufmConnectionDetails{}
	err := json.Unmarshal(connectionDetails, connDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ufm connection details %s", string(connectionDetails))
	}
	// check required fields
	if connDetails.Username == "" || connDetails.Password == "" || connDetails.Address == "" {
		return nil, fmt.Errorf(`failed to find one or more required filed of ["username", "password", "address"]`)
	}

	// set httpSchema and port to ufm default if missing
	if connDetails.HttpSchema == "" {
		connDetails.HttpSchema = "https"
	}
	if connDetails.Port == 0 {
		if strings.ToLower(connDetails.HttpSchema) == "https" {
			connDetails.Port = 443
		} else {
			connDetails.Port = 80
		}
	}

	return &ufmSubnetMangerClient{connectionDetails: connDetails}, nil
}
func (u *ufmSubnetMangerClient) Connect() error {
	// Create client and make request to ufm version api to check connection data are correct
	u.createUfmClient()
	if err := u.executeRequest("GET", "/ufmRest/app/ufm_version", 200, nil); err != nil {
		return fmt.Errorf("failed to connect to fum subnet manger %v", err)
	}

	return nil
}

func (u *ufmSubnetMangerClient) AddPKey(guid, pKey string) error {
	body := []byte(fmt.Sprintf(`{"pkey": "%s", "index0": true, "ip_over_ib": true, "guids": ["%s"]}`, pKey, guid))
	if err := u.executeRequest("POST", "/ufmRest/resources/pkeys", 200, body); err != nil {
		return fmt.Errorf("failed to add PKey %v, guid %v, error %v", pKey, guid, err)
	}

	return nil
}

func (u *ufmSubnetMangerClient) RemovePKey(guid, pKey string) error {
	path := fmt.Sprintf("/ufmRest/resources/pkeys/%s/guids/%s", pKey, guid)
	if err := u.executeRequest("DELETE", path, 200, nil); err != nil {
		return fmt.Errorf("failed to delete PKey %v, guid %v, error %v", pKey, guid, err)
	}

	return nil
}

func (u *ufmSubnetMangerClient) createUfmClient() {
	client := http.Client{Transport: http.DefaultTransport}
	if strings.ToLower(u.connectionDetails.HttpSchema) == "https" {
		client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	u.client = client
}

func (u *ufmSubnetMangerClient) createUfmRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request object %v", err)
	}
	req.SetBasicAuth(u.connectionDetails.Username, u.connectionDetails.Password)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return req, nil
}

func (u *ufmSubnetMangerClient) createUfmUrl(path string) string {
	return fmt.Sprintf("%s://%s:%d%s", u.connectionDetails.HttpSchema, u.connectionDetails.Address, u.connectionDetails.Port, path)
}

func (u *ufmSubnetMangerClient) executeRequest(method, path string, expectedStatusCode int, body []byte) error {
	url := u.createUfmUrl(path)
	req, err := u.createUfmRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("faied request %v", err)
	}
	if resp.StatusCode != expectedStatusCode {
		return fmt.Errorf("failed request ufm subnet manger with status code %v, expected status code %v, status %s", resp.StatusCode, expectedStatusCode, resp.Status)
	}

	return nil
}
