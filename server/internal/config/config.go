package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env           string
	Port          string
	DatabaseURL   string
	RedisURL      string
	PublicBaseURL string

	JWTSecret   []byte
	PhonePepper []byte
	DataKey     []byte
	IPPepper    []byte

	SMSProvider   string
	MSG91AuthKey  string
	MSG91Template string
	MSG91SOSFlow  string
	OTPDevCode    string

	TwilioSID   string
	TwilioToken string
	TwilioFrom  string

	R2AccountID   string
	R2AccessKey   string
	R2SecretKey   string
	R2Bucket      string
	R2PublicURL   string
	NSFWEndpoint  string
	AnthropicKey  string
	ExpoPushToken string

	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string

	AdminBootstrapEmail string
}

func (c *Config) IsProd() bool { return c.Env == "production" }
func (c *Config) IsDev() bool  { return c.Env == "development" }

func Load() (*Config, error) {
	c := &Config{
		Env:           get("APP_ENV", "development"),
		Port:          get("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		RedisURL:      get("REDIS_URL", "redis://localhost:56379/0"),
		PublicBaseURL: strings.TrimRight(get("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),

		SMSProvider:   get("SMS_PROVIDER", "dev"),
		MSG91AuthKey:  os.Getenv("MSG91_AUTH_KEY"),
		MSG91Template: os.Getenv("MSG91_OTP_TEMPLATE_ID"),
		MSG91SOSFlow:  os.Getenv("MSG91_SOS_FLOW_ID"),
		OTPDevCode:    os.Getenv("OTP_DEV_CODE"),
		TwilioSID:     os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioToken:   os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioFrom:    os.Getenv("TWILIO_FROM"),

		R2AccountID:  os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKey:  os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:     os.Getenv("R2_BUCKET"),
		R2PublicURL:  strings.TrimRight(os.Getenv("R2_PUBLIC_URL"), "/"),
		NSFWEndpoint: os.Getenv("NSFW_ENDPOINT"),
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),

		ExpoPushToken: os.Getenv("EXPO_ACCESS_TOKEN"),

		LiveKitURL:       get("LIVEKIT_URL", "ws://localhost:7880"),
		LiveKitAPIKey:    get("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret: get("LIVEKIT_API_SECRET", "secret"),

		AdminBootstrapEmail: os.Getenv("ADMIN_BOOTSTRAP_EMAIL"),
	}

	var errs []error
	c.JWTSecret = secret("JWT_SECRET", c, &errs)
	c.PhonePepper = secret("PHONE_PEPPER", c, &errs)
	c.IPPepper = secret("IP_PEPPER", c, &errs)
	c.DataKey = key32("DATA_KEY", c, &errs)

	if c.IsProd() {
		if c.DatabaseURL == "" {
			errs = append(errs, errors.New("DATABASE_URL is required in production"))
		}
		if c.SMSProvider == "dev" {
			errs = append(errs, errors.New("SMS_PROVIDER=dev is not allowed in production"))
		}
		if c.OTPDevCode != "" {
			errs = append(errs, errors.New("OTP_DEV_CODE must be empty in production"))
		}
	}
	return c, errors.Join(errs...)
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Dev/test fall back to fixed, obviously-insecure values so the app runs with zero setup;
// production refuses to start without real secrets.
func secret(k string, c *Config, errs *[]error) []byte {
	v := os.Getenv(k)
	if v == "" {
		if c.IsProd() {
			*errs = append(*errs, fmt.Errorf("%s is required in production", k))
		}
		return []byte("insecure-dev-" + strings.ToLower(k))
	}
	if c.IsProd() && len(v) < 32 {
		*errs = append(*errs, fmt.Errorf("%s must be at least 32 characters", k))
	}
	return []byte(v)
}

func key32(k string, c *Config, errs *[]error) []byte {
	v := os.Getenv(k)
	if v == "" {
		if c.IsProd() {
			*errs = append(*errs, fmt.Errorf("%s is required in production", k))
		}
		return []byte("insecure-dev-data-key-32-bytes!!")
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(b) != 32 {
		*errs = append(*errs, fmt.Errorf("%s must be 32 bytes, base64-encoded", k))
	}
	return b
}
