{
  lib,
  runCommand,
  nix-oci,
}:

{
  name,
  image,
  predicateType,
  useOciArtifacts ? false,
  attestations,
}:

runCommand name
  {
    nativeBuildInputs = [ nix-oci ];
  }
  ''
    nix-oci add-attestation \
      --output $out \
      --predicate-type=${lib.escapeShellArg predicateType} \
      --oci-artifact=${lib.boolToString useOciArtifacts} \
      ${lib.escapeShellArg image} \
      ${lib.escapeShellArgs attestations}
  ''
