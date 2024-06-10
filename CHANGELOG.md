# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Send pushes for all user devices

## [0.0.13] - 2024-03-26

### Added
- Collecting system stats

## [0.0.12] - 2024-03-19

### Changed 
- Update history schema to store null clicked_at instead of 0000-00-00

## [0.0.11] - 2024-03-06

### Changed
- Update platform events library to collect nats metrics

## [0.0.10] - 2024-02-08

### Changed
- Fix message id marshaling

## [0.0.8] - 2024-02-05

### Added
- Refactor sending pushes

## [0.0.7] - 2024-01-29

### Added
- Send notifications v2. Extend message with APNSConfig and Data fields.
- Add uuid for messages to mark them as read.

## [0.0.6] - 2023-11-03

### Changed
- Decrease log level on getting push token

## [0.0.5] - 2023-10-19

### Added
- Idempotent key for sending to avoid duplicates in pushes
- Limiter for 1 push in minute

## [0.0.4] - 2023-10-06

### Changed
- Skip sending push on getting token error

## [0.0.3] - 2023-08-26

### Fixed
- Fixed GITHUB_TOKEN argument passing in the Dockerfile

## [0.0.2] - 2023-08-26

### Fixed
- Fixed missed go.sum file in the git

## [0.0.1] - 2023-07-27

### Added
- Added skeleton app
- Basic implementation for push sending
