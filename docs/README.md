# ThinkPixelMP documentation

This directory contains the normative architecture, security, interoperability, and operational contracts for ThinkPixelMP.

## Structure

- `adr/` records durable architectural decisions and their consequences.
- `architecture/` defines system context, ownership, and trust boundaries.
- `contracts/` defines domain and integration contracts.
- `../api/schemas/` contains the normative JSON Schema 2020-12 contracts, including the closed artifact-descriptor union and immutable lock graph.
- `security/` defines threats, invariants, data handling, and hostile-input controls.
- `operations/` defines availability, capacity, and supported-version expectations.
- `evidence/` records phase-exit reviews and reproducible acceptance evidence.

Normative terms such as **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are interpreted as described by RFC 2119 and RFC 8174 when written in uppercase.

`PLAN.md` remains the implementation contract until release-candidate closure. Durable decisions belong here and in numbered ADRs.

Data handling follows the shared ThinkPixel C0–C3 classification defined in [security/data-classification.md](security/data-classification.md).
