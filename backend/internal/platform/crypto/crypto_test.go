package crypto

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword("correct-horse-battery-staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("expected correct password to verify")
	}

	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Fatal("expected incorrect password to fail verification")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := []byte(`{"password":"super-secret"}`)
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ciphertext == string(plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("round trip mismatch: got %q want %q", decrypted, plaintext)
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)

	ciphertext, _ := enc.Encrypt([]byte("hello"))
	tampered := ciphertext[:len(ciphertext)-2] + "00"

	if _, err := enc.Decrypt(tampered); err == nil {
		t.Fatal("expected tampered ciphertext to fail authentication")
	}
}

func TestNewEncryptorRejectsWrongKeyLength(t *testing.T) {
	if _, err := NewEncryptor("dG9vc2hvcnQ="); err == nil {
		t.Fatal("expected short key to be rejected")
	}
}
