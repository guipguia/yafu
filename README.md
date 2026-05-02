# YAFU (Yet Another Flux UI)

YAFU is an OpenSource, modern web-based UI for managing and visualizing [FluxCD](https://fluxcd.io/) GitOps workflows. 

## Features

- **Visual Dashboard**: View the status of your Flux Kustomizations, HelmReleases, and Sources in real time.
- **Modern Interface**: Designed for clarity and ease of use.
- **Easy Deployment**: Simple Kubernetes deployment alongside your existing Flux installation.

## Getting Started

Currently under active development. You can deploy YAFU using the provided charts or manifests in the `deploy/` directory.

### Prerequisites

- A running Kubernetes cluster.
- FluxCD installed on the cluster.

## Development

To run the project locally for development, you can use the provided Makefile:

```bash
# Run the development environment
make dev
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for more details on how to get started.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Security

Please refer to our [Security Policy](SECURITY.md) to safely report vulnerabilities.

## Code of Conduct

Please adhere to our [Code of Conduct](CODE_OF_CONDUCT.md) when interacting with the community.
