import {
  GetAuthState, GetRunStates, ListTunnels, OpenLogDir,
  RetryVerify, StartTunnel, StopTunnel, WriteOpLog,
  CreateTunnel, DeleteTunnel, GetTunnelDetail,
  GetIngressConfig, SaveIngressConfig,
  EnsureCNAME, DeleteDNSByName, ListZones, ListDNSRecords,
  VerifyAndSaveToken,
} from '../../wailsjs/go/main/App'

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
