# pipelineiq shared library

Jenkins loads this as the implicit global library `pipelineiq`.

- `timedStage` wraps `stage` and appends one record to `stages.json`.
- `reportBuild` uploads the build. It never fails the build.

`shared-library/test` is a small Maven project. It uses JenkinsPipelineUnit 1.31. Run it with JDK 21:

```bash
mvn -B -f shared-library/test/pom.xml test
```

That needs Maven and network access to Maven Central and `https://repo.jenkins-ci.org/releases/`. This sandbox did not run it.
