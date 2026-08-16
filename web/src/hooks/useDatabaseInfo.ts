import { useQuery } from '@connectrpc/connect-query'
import { getDatabaseInfo } from '../gen/purser/database/v1/database-DatabaseService_connectquery'

// useDatabaseInfo wraps DatabaseService.GetDatabaseInfo (see
// proto/purser/database/v1/database.proto) — the Database tab's (#613)
// read of driver/version/storage size/per-collection counts. Same
// one-hook-per-RPC shape as useSettings.
export function useDatabaseInfo() {
  return useQuery(getDatabaseInfo, {})
}
