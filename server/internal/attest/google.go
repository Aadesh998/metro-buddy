package attest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"golang.org/x/oauth2/google"
)

// PlayIntegrity decodes tokens server-side through Google's API with a service account.
type PlayIntegrity struct {
	Package     string
	Credentials []byte
	BaseURL     string
}

type integrityVerdict struct {
	TokenPayloadExternal struct {
		RequestDetails struct {
			RequestPackageName string `json:"requestPackageName"`
			Nonce              string `json:"nonce"`
		} `json:"requestDetails"`
		AppIntegrity struct {
			AppRecognitionVerdict string `json:"appRecognitionVerdict"`
		} `json:"appIntegrity"`
		DeviceIntegrity struct {
			DeviceRecognitionVerdict []string `json:"deviceRecognitionVerdict"`
		} `json:"deviceIntegrity"`
	} `json:"tokenPayloadExternal"`
}

func (p *PlayIntegrity) Verify(ctx context.Context, token, nonce string) error {
	creds, err := google.CredentialsFromJSON(ctx, p.Credentials, "https://www.googleapis.com/auth/playintegrity")
	if err != nil {
		return err
	}
	base := p.BaseURL
	if base == "" {
		base = "https://playintegrity.googleapis.com"
	}
	body, _ := json.Marshal(map[string]string{"integrity_token": token})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v1/%s:decodeIntegrityToken", base, p.Package), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	tok, err := creds.TokenSource.Token()
	if err != nil {
		return err
	}
	tok.SetAuthHeader(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("play integrity status %d", resp.StatusCode)
	}
	var v integrityVerdict
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return err
	}
	return checkVerdict(v, p.Package, nonce)
}

func checkVerdict(v integrityVerdict, pkg, nonce string) error {
	t := v.TokenPayloadExternal
	if t.RequestDetails.Nonce != nonce || t.RequestDetails.RequestPackageName != pkg {
		return errors.New("nonce or package mismatch")
	}
	if t.AppIntegrity.AppRecognitionVerdict != "PLAY_RECOGNIZED" {
		return errors.New("app not recognised")
	}
	if !slices.Contains(t.DeviceIntegrity.DeviceRecognitionVerdict, "MEETS_DEVICE_INTEGRITY") {
		return errors.New("device integrity not met")
	}
	return nil
}
