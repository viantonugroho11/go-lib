## go-lib

A collection of small Go libraries intended to be reusable across projects.

### Modules
- `kafka`: Sarama-based Kafka Producer and Consumer helpers
  - Generic Producer with simple options and ENV support
  - Consumer Group wrapper with typed handler adapter (auto JSON unmarshal)
  - See `kafka/README.md` for full usage
- `xlog`: Lightweight zap-based logging helper
  - Options/ENV configuration, stdout/stderr/file (rotation), field helpers
  - Context field extractor, global accessors `Logger()` and `SugaredLogger()`
  - See `xlog/README.md` for full usage

### Requirements
- Go 1.23+ (adjust to your environment if necessary)

### Getting Started
- For Kafka usage, review `kafka/README.md` and the `example/` folder
- For Logging, review `xlog/README.md` and `xlog/example/`

### License
MIT (if unspecified, adapt accordingly)

