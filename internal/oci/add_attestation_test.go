package oci_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/systemstart/nix-oci/internal/oci"
	"github.com/opencontainers/go-digest"
)

func TestAddAttestation(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	// The source image to which we add an attestation
	source := filepath.Join(base, "source")
	if err := oci.Write(source, oci.ImageOptions{
		Roots: []string{storePath(t, base, "aaa-x", "x")},
		MaxLayers: 100,
		User: "1000:1000",
		Architecture: "amd64",
		OS: "linux",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The attestation to add
	attestationData := filepath.Join(base, "attestation.json")
	data, err := json.Marshal(map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"predicateType": "https://spdx.dev/Document",
	})
	if err != nil {
		t.Fatalf("failed to write attestation")
	}
	if err := os.WriteFile(attestationData, data, 0o555); err != nil {
		t.Fatalf("write: %v", err)
	}
	attestations := []string{attestationData}

	target := filepath.Join(base, "target")
	if err := oci.AddAttestations(target, source, "https://spdx.dev/Document", attestations); err != nil {
		t.Fatalf("AddAttestation: %v", err)
	}

	// Assert that the index contains precisely our image and its
	// attestation
	var index oci.Index
	unmarshalFile(t, filepath.Join(target, "index.json"), &index)

	if index.MediaType != oci.MediaTypeIndex {
		t.Errorf("index mediaType = %q", index.MediaType)
	}

	attestationSubject := index.Manifests[1]
	if (attestationSubject.Annotations["vnd.docker.reference.type"] != "attestation-manifest") {
		t.Fatalf("attestation index entry doesn't have the right metadata: %v", attestationSubject)
	}

	// Assert that the attestation manifest is correct
	var attestationManifest oci.Manifest
	if err := readBlob(target, attestationSubject.Digest, &attestationManifest); err != nil {
		t.Fatalf("could not read attestation manifest")
	}

	if (attestationManifest.ArtifactType != "application/vnd.docker.attestation.manifest.v1+json") {
		fmt.Printf("%+v", attestationManifest)
		t.Fatalf("attestation manifest does not have the right metadata: %v", attestationManifest)
	}

	// Ensure that the manifest points to a correct in-toto attestation
	var attestation map[string]any
	if err := readBlob(target, attestationManifest.Layers[0].Digest, &attestation); err != nil {
		t.Fatalf("could not read attestation")
	}

	if (attestation["_type"] != "https://in-toto.io/Statement/v1") {
		t.Fatalf("incorrect attestation type")
	}
}

// readBlob reads a JSON blob by digest from a layout directory.
func readBlob(dir string, dgst digest.Digest, v any) error {
	return readJSONFile(filepath.Join(dir, "blobs", "sha256", dgst.Encoded()), v)
}

func readJSONFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	return nil
}
