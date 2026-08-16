import type { GetDatabaseInfoResponse as ProtoDatabaseInfo } from '../../gen/purser/database/v1/database_pb'
import type { DatabaseInfo } from '../../types'

// databaseFromProto converts one wire purser.database.v1.GetDatabaseInfoResponse
// into the plain DatabaseInfo app code is written against
// (web/src/types/index.ts) — mirrors settingFromProto/jobFromProto's shape.
// bigint fields (storageSizeBytes, each collectionCounts value) become
// number — see DatabaseInfo's own doc comment for why that's safe here.
export function databaseInfoFromProto(info: ProtoDatabaseInfo): DatabaseInfo {
  const collectionCounts: Record<string, number> = {}
  for (const [collection, count] of Object.entries(info.collectionCounts)) {
    collectionCounts[collection] = Number(count)
  }
  return {
    driver: info.driver,
    version: info.version,
    storageSizeBytes: Number(info.storageSizeBytes),
    collectionCounts,
  }
}
