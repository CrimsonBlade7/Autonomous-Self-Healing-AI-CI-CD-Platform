# Autonomous-CI-Platform

## Notes
- Dockerfile is supplied by the user.
- Fake .env variables are supplied by the user at internal\config\test-env-vars.txt
- Linter files are supplied by the user and are run if they exist.
- May lose closed pull requests.
- Always run the orchestrator from ".\orchestrator".

## Internal Contract

### Orchestrator -> AI Engine
The orchestrator sends an http request to the local port assigned to the AI Engine. See Job-Type headers for more info.

#### Headers
- HMAC-Signature-256: `<sha256 hmac signature>`
- Accept: application/json
- Job-Type: `<see below>`

The Job-Type header is one of:
- open:     Start a workflow when a pr opens.
- close:    Pull request has been closed. Merge is unspecified.
- logs:     Return the logs of the last test run.
- edit:     Update pr information.
- sync:     Update branch head

#### Data
The data being sent is contained within a AIEngineRequest struct as a JSON with the following fields:
```go
Stdout    string
Stderr    string
StartTime time.Time
EndTime   time.Time
Errors    string      // Compile or entry command errors
Status    string      // One of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
OOMKilled bool        // Killed: out of memory
ExitCode  int
```

### AI Engine -> Orchestrator
The AI Engine sends an http request to the local port assigned to the orchestrator. The AI Engine sends logs or a done signal.

#### Headers
- HMAC-Signature-256: `<sha256 hmac signature>`
- Accept: application/json

#### Data
The data being recieved is a JSON with that can be unmarshalled into a AIEngineResponse struct the following fields:
```go
Wfid          int
PullRequest   PullRequest
TestName      string
Tests         []byte      // Tests are ignored if Done.
Done          bool        // Always accompanied by summary.
Summary       string
```

## Configurable Environment Variables
```go
WsDir string

OrchRootDir                 string
Port                        string = "8080"
GithubToken                 string
RepositoryUrl               string
GithubSecret                string
AIEngineSecret              string
AIEnginePort                string = "8000"
AiEngineRequestTimeout      int    = 5   // seconds
ServerShutdownTimeout       int    = 30  // seconds
ReadHeaderTimeout           int    = 2   // seconds
WriteTimeout                int    = 5   // seconds
ContainerTimeout            int    = 10  // minutes
AIEngineRequestCloseTimeout int    = 10  // seconds
DockerStartTimeout          int    = 10  // seconds
ContainerMemoryCap          int    = 512 // MB
MaxTestPatchingAttempts     int    = 10
TestingEnvSlice             []string
```