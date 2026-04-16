# proto-font

## Instructions

Make sure you create a setup.sh, build.sh, test.sh, and LET_IT_RIP.sh that contain all project setup scripts/commands used - NEVER build/test/run the code in this repo outside of these scripts, NEVER commit or push without running these either. Make them idempotent so that each build.sh can run setup.sh and skip things already set up, each test.sh can run build.sh, each LET_IT_RIP runs test.sh

use go1.26

Encode the latest versions of the OpenType, TrueType, and woff font formats into protobuf messages, similarly to how we did it in this project: https://github.com/accretional/mime-proto/blob/main/pb/proto/openformat/v1/docx.proto

try to use protos you find in https://github.com/google/fonts if they are related to the font formats or Noto, we are going to set upa. validation/test set with all the noto fonts https://github.com/google/fonts/blob/b669b896a75927719f611ac76f329bbeab32dc61/lang/Lib/gflanguages/languages_public.proto https://github.com/google/fonts/blob/b669b896a75927719f611ac76f329bbeab32dc61/axisregistry/Lib/axisregistry/axes.proto

here's some stuff from other repos

<details><summary>googlefonts files with .proto, mangled formatting, try github googlefonts/gftools or similar</summary>
17 files  (568 ms)
17 files
in
googlefonts (press backspace or delete to remove)
Files with identical content are grouped together.
googlefonts/gftools · Lib/gftools/axes.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
// GF Axis Registry Protos
// An axis in the GF Axis Registry
message AxisProto {
  // Axis tag
  optional string tag = 1;
googlefonts/PFE-analysis · analysis/result.proto

    Protocol Buffer
    ·
    0 (0)

// Proto definition of used to store the results of
// the analysis.
syntax = "proto3";
package analysis;
message AnalysisResultProto {
googlefonts/gftools · Lib/gftools/designers.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
// GF Designer Profile Protos
// A designer listed on the catalog:
message DesignerInfoProto {
  // Designer or typefoundry name:
  optional string designer = 1;
googlefonts/gf-metadata · resources/protos/designers.proto

googlefonts/gf-metadata · resources/scripts/embed_data.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
message FloatVecProto {
    repeated float value = 1;
}
message MetadataProto {
googlefonts/PFE-analysis · analysis/pfe_methods/unicode_range_data/slicing_strategy.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto3";
package analysis.pfe_methods.unicode_range_data;
message SlicingStrategy {
  repeated Subset subsets = 1;
}
googlefonts/gftools · Lib/gftools/fonts_public.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * Open Source'd font metadata proto formats.
 */
package google.fonts_public;
googlefonts/gf-metadata · resources/protos/fonts_public.proto

googlefonts/lang · Lib/gflanguages/languages_public.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * languages/regions/scripts proto formats.
 */
package google.languages_public;
googlefonts/gf-metadata · resources/protos/languages_public.proto

googlefonts/FontClassificationTool · fonts_public.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * Open Source'd font metadata proto formats.
 */
package google.fonts;
googlefonts/gf-metadata · resources/protos/axes.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
// GF Axis Registry Protos
// An axis in the GF Axis Registry
message AxisProto {
  // Axis tag
  optional string tag = 1;
googlefonts/PFE-analysis · analysis/page_view_sequence.proto

    Protocol Buffer
    ·
    0 (0)

// Proto format used by the open source incxfer analysis code.
syntax = "proto3";
package analysis;
message PageContentProto {
  string font_name = 1;
googlefonts/gftools · Lib/gftools/knowledge.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * Proto definitions for Fonts Knowledge metadata in the filesystem.
 */
package fonts;
googlefonts/gf-metadata · resources/protos/knowledge.proto

googlefonts/fontbakery-dashboardArchived · containers/base/protocolbuffers/shared.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto3";
package fontbakery.dashboard;
message File {
  string name = 1;
  bytes data = 2;
googlefonts/fontbakery-dashboardArchived · containers/base/protocolbuffers/messages.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto3";
import "google/protobuf/any.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";
import public "shared.proto";

</details>

do the same in https://github.com/googlefonts/axisregistry

get the fonts out of https://github.com/google/material-design-icons

document and build a client for the google fonts api with documentation at https://developers.google.com/fonts/docs/developer_api#api_url_specification 

integrate https://github.com/googlefonts/lang

Do this

```
you can download all Google Fonts in a simple ZIP snapshot (over 1GB) from https://github.com/google/fonts/archive/main.zip
Sync With Git

You can also sync the collection with git so that you can update by only fetching what has changed. To learn how to use git, GitHub provides illustrated guides, a youtube channel, and an interactive learning site. Free, open-source git applications are available for Windows and Mac OS X.
```

Go through https://developers.google.com/fonts/faq and document anything interesting in docs/googlefaq-tldr.md

do the same in https://googlefonts.github.io/gf-guide/ document it in docs/gf-guide-tldr.md

Do the same with https://github.com/orgs/googlefonts/repositories, don't go crazy importing random fonts there tho, docs/googlefonts-repos-tldr/

do 
