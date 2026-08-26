import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './layout/AppShell'
import { CachePage } from './pages/cache/CachePage'
import { FlowPage } from './pages/flow/FlowPage'
import { SchemasPage } from './pages/schemas/SchemasPage'
import { SchedulesPage } from './pages/schedules/SchedulesPage'
import { ScriptsPage } from './pages/scripts/ScriptsPage'

export function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/alpha/flow" replace />} />
        <Route path="flow" element={<Navigate to="/alpha/flow" replace />} />
        <Route path="scripts" element={<Navigate to="/alpha/scripts" replace />} />
        <Route path="schemas" element={<Navigate to="/alpha/schemas" replace />} />
        <Route path="schedules" element={<Navigate to="/alpha/schedules" replace />} />
        <Route path="cache" element={<Navigate to="/alpha/instances" replace />} />
        <Route path="instances" element={<Navigate to="/alpha/instances" replace />} />
        <Route path=":realm/flow" element={<FlowPage />} />
        <Route path=":realm/flow/*" element={<FlowPage />} />
        <Route path=":realm/scripts" element={<ScriptsPage />} />
        <Route path=":realm/scripts/:scriptId" element={<ScriptsPage />} />
        <Route path=":realm/schemas" element={<SchemasPage />} />
        <Route path=":realm/schemas/:schemaId" element={<SchemasPage />} />
        <Route path=":realm/schedules" element={<SchedulesPage />} />
        <Route path=":realm/schedules/:scheduleId" element={<SchedulesPage />} />
        <Route path=":realm/cache" element={<Navigate to="../instances" replace />} />
        <Route path=":realm/instances" element={<CachePage />} />
      </Route>
    </Routes>
  )
}

export default App
