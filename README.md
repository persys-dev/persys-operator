# persys-operator

`persys-operator` is the Kubernetes operator workspace for Persys-related CRDs/controllers.

## Current Status

- Operator scaffolding exists.
- Not part of the default local compose runtime path.
- Main control-plane runtime currently relies on gateway/scheduler/agent/forgery services.

## Typical Dev Commands

```bash
cd persys-operator
make test
make build
```

If deploying to a cluster, follow the generated operator targets in the Makefile (`make install`, `make deploy`).
