package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const SessionCookie = "anzhen_session"

type Identity struct {
	Subject string `json:"subject"`
	Role    string `json:"role"`
	Expires int64  `json:"expires"`
}

type Auth struct {
	key        []byte
	secure     bool
	doctorCode string
}

func New(secret, doctorCode string, secure bool) (*Auth, error) {
	if strings.TrimSpace(secret) == "" {
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		secret = hex.EncodeToString(random[:])
	}
	return &Auth{key: []byte(secret), secure: secure, doctorCode: strings.TrimSpace(doctorCode)}, nil
}

func (a *Auth) Issue(w http.ResponseWriter, role, subject string) (Identity, error) {
	identity := Identity{Subject: subject, Role: role, Expires: time.Now().Add(7 * 24 * time.Hour).Unix()}
	token, err := a.sign(identity)
	if err != nil {
		return Identity{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Unix(identity.Expires, 0),
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return identity, nil
}

func (a *Auth) Guest(w http.ResponseWriter) (Identity, error) {
	return a.Issue(w, "patient", "patient_"+randomID())
}

func (a *Auth) Doctor(w http.ResponseWriter, code string) (Identity, error) {
	if a.doctorCode == "" || !hmac.Equal([]byte(strings.TrimSpace(code)), []byte(a.doctorCode)) {
		return Identity{}, errors.New("医生访问口令不正确")
	}
	return a.Issue(w, "doctor", "doctor_lin_xia")
}

func (a *Auth) Identity(r *http.Request) (Identity, error) {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil {
		return Identity{}, errors.New("请先进入患者咨询室或登录医生端")
	}
	return a.verify(cookie.Value)
}

func (a *Auth) Require(r *http.Request, role string) (Identity, error) {
	identity, err := a.Identity(r)
	if err != nil {
		return Identity{}, err
	}
	if identity.Role != role {
		return Identity{}, errors.New("没有此操作权限")
	}
	return identity, nil
}

func (a *Auth) sign(identity Identity) (string, error) {
	payload, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (a *Auth) verify(token string) (Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Identity{}, errors.New("登录状态无效")
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Identity{}, errors.New("登录状态无效")
	}
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return Identity{}, errors.New("登录状态无效")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Identity{}, errors.New("登录状态无效")
	}
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil || identity.Subject == "" || identity.Expires < time.Now().Unix() {
		return Identity{}, errors.New("登录状态已过期")
	}
	return identity, nil
}

func randomID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return hex.EncodeToString([]byte(time.Now().Format("20060102150405")))
}
