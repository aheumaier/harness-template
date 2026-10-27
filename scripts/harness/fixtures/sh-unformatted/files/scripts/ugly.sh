#!/usr/bin/env bash
# Deliberate violation: hand-aligned, one-line if, tab indentation.
if [ -n "${1:-}" ];then echo "$1";fi
for f in a b
do
	echo "$f"
done
