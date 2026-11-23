# Tasks

## 1. Images

- [x] 1.1 Decision record 0020; `.github/workflows/images.yml` (`deploy/demo/compose_test.go`: workflow definition).

## 2. Demo

- [x] 2.1 `deploy/demo/compose.yaml`, `demo.env.example`, `.gitignore` (`compose_test.go`: rendered with secrets, missing secret).
- [x] 2.2 Run the demo from locally built images; README section.

## Results

- `TestRunningDemo` from locally built images: every service running or healthy, and the dashboard's proxy reaching the registry through the gateway, 31 s after `up`.
- Bootstrap from empty volumes: up in 7 s; EuroSAT downloaded and version 1 trained (3 epochs, holdout accuracy 0.9032), validated, and in shadow 483 s later. The first attempt failed with `Permission denied: '/data/eurosat'`: a new named volume takes its ownership from the image's directory, and the non-root image had none. The image now creates `/data` owned by its user.
