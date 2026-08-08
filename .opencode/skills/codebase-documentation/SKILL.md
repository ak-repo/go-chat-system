---
name: codebase-documentation
description: Generate accurate architecture and feature documentation from the current go-chat-system repository.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Codebase Documentation

Documentation must be derived from current source code.

## Source priority

Use this order:

1. Current source code
2. Current migrations
3. Current configuration
4. Current tests
5. Existing documentation

Existing documentation is supporting context only.

If documentation conflicts with current code, current code wins.

## Architecture discovery

Inspect:

cmd/server/

internal/domain/
internal/platform/
internal/repository/
internal/service/
internal/shared/

internal/transport/injector/
internal/transport/middleware/
internal/transport/routes/
internal/transport/websocket/
internal/transport/wrapper/

migrations/

web/src/api/
web/src/context/
web/src/pages/

## Feature classification

Every feature must be classified as one of:

### Implemented

Complete execution path exists.

### Partial

Important pieces exist but the complete user flow does not.

### Scaffolded

Models/types/infrastructure exist but the feature is not operational.

### Missing

No meaningful implementation exists.

Never classify something as implemented from a model or TODO alone.

## Feature verification

For backend features inspect:

route
-> handler/service
-> repository
-> database

For realtime features inspect:

route/authentication
-> WebSocket handler
-> client/hub
-> service
-> persistence
-> frontend socket handling

For frontend features inspect:

page
-> context/state
-> API/socket client
-> backend contract

## Recommended document structure

# Project Overview

# Technology Stack

# Repository Structure

# Backend Architecture

# Frontend Architecture

# Database Architecture

# Authentication Flow

# REST API

# WebSocket Architecture

# Implemented Features

# Partial / Scaffolded Features

# Configuration

# Development Workflow

# Testing

# Deployment Architecture

# Current Limitations

# Important Technical Decisions

## Accuracy rules

Do not:

- invent endpoints
- invent tables
- assume TODOs are implemented
- copy stale documents without verifying them
- describe generic Go/React patterns unless actually used

Use exact file paths where they help engineers locate implementation.