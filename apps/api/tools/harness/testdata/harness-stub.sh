#!/usr/bin/env bash
# A .bin/harness stand-in for the wrapper tests: prints $STUB_OUTPUT to stderr and exits $STUB_EXIT.
printf '%s\n' "$STUB_OUTPUT" >&2
exit "${STUB_EXIT:?set STUB_EXIT}"
