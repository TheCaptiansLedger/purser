# Cross-Cutting Feature Design

Before implementing any new feature, reason through how it applies to **every content type**: `movie`, `tv`, `music`, `adult`, `jav`.

If a feature only touches one module, that is a red flag — either the feature belongs in shared infrastructure or the design needs to be reconsidered.

## Questions to Ask Before Every Feature

- How does this work for movies? TV? Music? Adult? JAV?
- Does it belong in a shared component/API/service, or does each module need its own implementation?
- If the answer differs per content type, is the difference encoded in data (content_type field, config) rather than duplicated code?

## Never Snowflake a Module

Logic that appears in one content type's code but would also be needed in another must live in shared code. The content type is a data dimension, not a reason to fork the architecture.
