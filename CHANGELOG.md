# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
