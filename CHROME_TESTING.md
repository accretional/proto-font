# Chrome Testing

Visual validation for proto-font using headless Chrome screenshots. Each font
fixture under `data/fonts/` gets rendered in a browser and captured as a PNG.
The results live in `chrome-testing/screenshots/` and the best ones go into
`README.md`.

## Setup

Pull the `testing/` folder from
[accretional/chromerpc](https://github.com/accretional/chromerpc) and place it
in this repo as `chrome-testing/`. You only need that folder and not the whole
repo.

## Generating HTML samples

Inside `chrome-testing/`, write a script that walks `data/fonts/` and produces
one HTML file per font. Each HTML file should load the font and display it at a
few sizes so rendering quality is visible. Keep the HTML files inside `chrome-testing/`.

Run the script to generate the HTML files.

## Taking screenshots

Run `chrome-testing/snap.sh` against the directory of HTML files you just
generated. It will produce one PNG per file and open them for you to inspect.
The script handles everything — Chrome, the server, the gRPC layer — you just
point it at the HTML directory and an output directory.

## Validation

Open every PNG and confirm the font is actually rendering. Anything blank or 
garbled means something went wrong upstream (bad fixture, unsupported format, 
encoding bug). Note any failures.

## Adding to README

Pick the screenshots that best represent the codec's output and add them to
`README.md` under `## UI validation samples`. Reference the files by
relative path from the repo root.
