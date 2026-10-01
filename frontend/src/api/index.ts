import {
  GetAuthState, GetRunStates, ListTunnels, OpenLogDir,
  RetryVerify, StartTunnel, StopTunnel, WriteOpLog,
  CreateTunnel, DeleteTunnel, GetTunnelDetail, GetTunnelToken,
  GetIngressConfig, SaveIngressConfig,
  EnsureCNAME, DeleteDNSByName, ListZones, ListDNSRecords,
  VerifyAndSaveToken,
} from '../../wailsjs/go/main/App'

export type { cloudflare } from '../../wailsjs/go/models'
export { auth } from '../../wailsjs/go/models'

export const api = {
  auth: {
    getState: GetAuthState,
    retry: RetryVerify,
    verify: VerifyAndSaveToken,
  },
  tunnel: {
    list: ListTunnels,
    runStates: GetRunStates,
    start: StartTunnel,
    stop: StopTunnel,
    create: CreateTunnel,
    remove: DeleteTunnel,
    detail: GetTunnelDetail,
    token: GetTunnelToken,
  },
  ingress: {
    get: GetIngressConfig,
    save: SaveIngressConfig,
  },
  dns: {
    zones: ListZones,
    records: ListDNSRecords,
    ensure: EnsureCNAME,
    remove: DeleteDNSByName,
  },
  system: {
    openLogDir: OpenLogDir,
    opLog: WriteOpLog,
  },
}
