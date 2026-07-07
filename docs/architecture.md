# Architecture

This document describes the high-level architecture of `go-seeder`.

## Core Engine
The root `seeder` package provides the engine for managing seed version state and executing the registered database migrations in order.

## Loaders
The `loader` package manages reading seeds from Go files, SQL files, and JSON files.

## Drivers
The `driver` package abstracts the underlying database (PostgreSQL, MySQL, MongoDB).
