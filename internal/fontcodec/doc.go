// Package fontcodec encodes and decodes font container formats (SFNT,
// WOFF 1.0, WOFF 2.0, TTC, EOT) to and from the openformat.v1 protobuf
// representation defined in proto/openformat/v1/font.proto.
//
// Round-trip contract: Decode always sets FontFileWithMetadata.raw_bytes;
// Encode returns raw_bytes verbatim when it is present. This guarantees
// byte-exact round-trips without depending on every table parser being
// perfect. When raw_bytes is empty (e.g. a freshly synthesised proto)
// Encode reconstructs the container from structured fields.
package fontcodec
