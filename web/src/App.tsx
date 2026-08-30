import { ConfigProvider, theme } from 'antd'
import { Dashboard } from './pages/Dashboard'

export default function App() {
  return (
    <ConfigProvider theme={{ algorithm: theme.defaultAlgorithm }}>
      <Dashboard />
    </ConfigProvider>
  )
}
