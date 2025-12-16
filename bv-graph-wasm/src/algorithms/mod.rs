//! Graph algorithm implementations.
//!
//! This module contains ports of the Go graph algorithms to Rust WASM.

pub mod articulation;
pub mod betweenness;
pub mod critical_path;
pub mod eigenvector;
pub mod hits;
pub mod kcore;
pub mod pagerank;
pub mod topo;

// Algorithm modules will be added as they're implemented:
// pub mod cycles;
// pub mod slack;
