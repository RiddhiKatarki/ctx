# Examples

This directory contains sample files for testing and demonstration.

## prompts.json

A sample prompt history file that can be supplied to `ctx export` via the `--prompts` flag:

```bash
ctx export --prompts examples/prompts.json
```

The file contains an array of `{role, content}` objects where role is `user`, `assistant`, or `system`.

## Generating a sample bundle

To generate a sample `.ctx` bundle from this repository itself:

```bash
# From the project root
ctx export --prompts examples/prompts.json --output examples/sample.ctx

# Inspect it
ctx inspect examples/sample.ctx

# Import it
ctx import examples/sample.ctx --outdir /tmp/sample-bundle/
```
