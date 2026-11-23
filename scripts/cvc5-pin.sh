#!/usr/bin/env bash

# Single source of the cvc5 pin CI installs: the official static release's zip, verified by
# its sha256. Bump the version and the digest together; the file behind a URL can change.
CVC5_VERSION="${CVC5_VERSION:-1.3.4}"
CVC5_FILE="cvc5-Linux-x86_64-static.zip"
CVC5_URL="${CVC5_URL:-https://github.com/cvc5/cvc5/releases/download/cvc5-${CVC5_VERSION}/${CVC5_FILE}}"
CVC5_SHA256="${CVC5_SHA256:-dcdbfada0ce493ee98259c0816e0daafc561c223aadb3af298c2968e73ea39c6}"
