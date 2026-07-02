# Contributing

## Language

Go (1.26+).

## Makefile

Use `make help` to see available targets. Define repetitive actions as Makefile targets.

## Testing

Write unit tests when adding or changing functionality. Run `make test` before submitting changes.

## Documentation

- `README.md` for general documentation
- `docs/` for supplementary documentation

## Dependencies

Minimize external dependencies. Always vendor with `make vendor`.

## Workflow

The project must build and all tests must pass before finishing work.

```bash
make fmt     # Format code
make lint    # Lint code
make test    # Run tests
make build   # Verify build
```
