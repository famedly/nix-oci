{
  lib,
  runCommand,
  nix-oci,
}:

{
  name,
  image,
  predicateType,
  attestations,
}:

runCommand name
  {
    nativeBuildInputs = [ nix-oci ];
  }
  ''
    nix-oci add-attestation \
      --output $out \
      --predicate-type ${lib.escapeShellArg predicateType} \
      ${lib.escapeShellArg image} \
      ${lib.escapeShellArgs attestations}
  ''
