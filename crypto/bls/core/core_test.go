package bls

import (
	"encoding/hex"
	"testing"
)

func TestHarmonyWireCompatibility(t *testing.T) {
	var secret SecretKey
	if err := secret.SetLittleEndian([]byte{1}); err != nil {
		t.Fatal(err)
	}

	const (
		wantSecret = "0100000000000000000000000000000000000000000000000000000000000000"
		wantPublic = "e500361ff315734cccd8f9b721ec159995e9e622be17afd41ac2f037a583e81b98c2320e0bf853a8f929e89e3d8ff504"
		wantSign   = "c2aba747613499b2e086c3bd9714f6da7159b3cb256d246f1c41dcdd61b00fd608e7caadef2376ed77e68ffc0b486113617d21668e56f930bdee39af0af1fcb42dc7396c5de18af5f2560dc6a11f5e9dd04f9df5e1ac68773d85c8585dcf9c11"
		wantHash   = "1d730dd8da233d2fccc254b9b3fece52a6f15dd4522f3cd600f14551b5cd76c95ea6ebc7ec077ccc83dead3b2b5b8a12f05b8492119c773c81eb5fb56060f585e711e763f4a85221a5ba72894b87fab619fab4a4c0e478969125c75a9216d093"
	)

	assertHex := func(name string, got []byte, want string) {
		t.Helper()
		if hex.EncodeToString(got) != want {
			t.Fatalf("%s changed:\n got %x\nwant %s", name, got, want)
		}
	}

	public := secret.GetPublicKey()
	hash, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatal(err)
	}
	assertHex("secret key", secret.Serialize(), wantSecret)
	assertHex("public key", public.Serialize(), wantPublic)
	assertHex("message signature", secret.Sign("harmony-bls-upgrade-compatibility").Serialize(), wantSign)
	assertHex("hash signature", secret.SignHash(hash).Serialize(), wantHash)
}
