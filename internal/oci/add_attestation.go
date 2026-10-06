package oci

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func AddAttestations(outputDir string, input string, predicateType string, attestations []string) error {
	blobDir := filepath.Join(outputDir, "blobs", "sha256")
	if err := os.MkdirAll(blobDir, dirMode); err != nil {
		return fmt.Errorf("create blob dir: %w", err)
	}

	manifests, err := combineInto(blobDir, []string{input})
	if err != nil {
		return err
	}

	index := Index{
		Versioned: Versioned{SchemaVersion: 2},
		MediaType: MediaTypeIndex,
		Manifests: manifests,
	}

	subject, err := getSubjectFromIndex(index)
	if err != nil {
		return err
	}

	var intotoSubjects []Descriptor
	for _, attestation := range attestations {
		attestation, err := writeAttestationBlob(blobDir, attestation, subject)
		if err != nil {
			return err
		}

		intotoSubjects = append(intotoSubjects, *attestation)
	}

	// Write a manifest for the given attestations
	attestationManifest := Manifest{
		MediaType: MediaTypeManifest, // Looks wrong, but that's the spec
		ArtifactType: "application/vnd.docker.attestation.manifest.v1+json",
		Config: v1.DescriptorEmptyJSON,
		Layers: intotoSubjects,
		Subject: subject,
	}
	attestationSubject, err := writeJSONBlob(blobDir, MediaTypeManifest, attestationManifest)
	if err != nil {
		return err
	}

	// Add additional metadata to the resulting subject object
	attestationSubject.Annotations = map[string]string{
		"vnd.docker.reference.digest": string(subject.Digest),
		"vnd.docker.reference.type": "attestation-manifest",
	}

	// The spec recommends setting this platform so that no runtime
	// accidentally attempts to execute the manifest
	dummyPlatform := v1.Platform{
		Architecture: "unknown",
		OS: "unknown",
	}
	attestationSubject.Platform = &dummyPlatform

	// Finally write a new image index that points to all the
	// resulting manifest files - even though attestations aren't
	// "images", they are stored in the same manifest index, with the
	// same MediaType - this spec is a little insane, but I didn't
	// write it, don't blame me.
	manifests = make([]v1.Descriptor, len(index.Manifests))
	copy(manifests, index.Manifests)
	manifests = append(manifests, attestationSubject)

	if err := writeIndex(outputDir, manifests...); err != nil {
		return fmt.Errorf("write image index: %w", err)
	}

	return writeLayoutMarker(outputDir)
}

func writeAttestationBlob(blobDir string, attestation string, subject *Descriptor) (d *Descriptor, err error) {
	var predicate map[string]any
	if err := readJSONFile(attestation, &predicate); err != nil {
		return nil, fmt.Errorf("attestation must be a correctly encoded JSON file")
	}

	intoto := attestationToInToTo(predicate, *subject)

	// Write the intoto file to a blob
	intotoSubject, err := writeJSONBlob(blobDir, "application/vnd.in-toto+json", intoto)
	if err != nil {
		return nil, err
	}

	return &intotoSubject, nil
}

func getSubjectFromIndex(index Index) (d *Descriptor, err error) {
	// Get the subset of descriptors that aren't attestations
	subjectDescriptors := slices.DeleteFunc(
		append([]Descriptor{}, index.Manifests...),
		func(manifest Descriptor) bool {
			descriptorType := manifest.Annotations["vnd.docker.reference.type"]
			return descriptorType == "attestation-manifest"
		},
	)

	if len(subjectDescriptors) != 1 {
		return nil, fmt.Errorf("must contain exactly one non-attestation index")
	}

	return &subjectDescriptors[0], nil
}

func attestationToInToTo(attestation map[string]any, subject Descriptor) map[string]any {
	d := string(subject.Digest)
	digest := strings.Split(d, ":")

	return map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"predicateType": "https://spdx.dev/Document",
		"subject": []map[string]any{{
			"name": "_",
			"digest": map[string]any{
				digest[0]: digest[1],
			},
		}},
		"predicate": attestation,
	}
}
