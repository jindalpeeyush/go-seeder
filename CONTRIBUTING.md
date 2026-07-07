# Contributing to go-seeder

Thank you for your interest in contributing to `go-seeder`! We welcome bug reports, feature requests, documentation improvements, and pull requests.

## How to Contribute

### 1. Reporting Bugs & Requesting Features
- Please search the open issues first to see if it has already been reported.
- If not, open a new issue using the appropriate template (Bug Report or Feature Request).

### 2. Development Setup
- Clone the repository: `git clone https://github.com/jindalpeeyush/go-seeder.git`
- Ensure you have Go 1.22+ installed locally.
- Run `go mod download` to fetch dependencies.

### 3. Running Tests
- Before submitting any changes, make sure all tests compile and pass:
  ```bash
  go test -v ./...
  ```

### 4. Code Standards
- Follow standard Go formatting guidelines (`go fmt`).
- Ensure all new public types, structs, interfaces, and methods are fully documented with GoDoc-compliant comments.
- Keep implementation details encapsulated within `internal/` packages while keeping the `pkg/` and root package public APIs clean.

### 5. Submitting a Pull Request
- Create a feature branch: `git checkout -b feature/my-new-feature`
- Commit your changes: `git commit -m "Add my new feature"`
- Push to your branch: `git push origin feature/my-new-feature`
- Open a Pull Request against the `main` branch. Provide a clear description of what your PR accomplishes.
