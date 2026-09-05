import { QueryClient, QueryClientProvider } from '@tanstack/preact-query'
import { render } from 'preact'
import { Login } from './components/Login'
import './styles.css'

const client = new QueryClient({
  defaultOptions: {
    queries: { refetchOnWindowFocus: false, retry: false },
  },
})

render(
  <QueryClientProvider client={client}>
    <Login />
  </QueryClientProvider>,
  document.getElementById('app')!,
)
