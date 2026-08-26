import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App.tsx'

function routerBaseName() {
  const href = document.querySelector('base')?.getAttribute('href')
  if (!href) {
    return '/'
  }
  const path = new URL(href, window.location.origin).pathname
  return path.length > 1 ? path.replace(/\/$/, '') : '/'
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter basename={routerBaseName()}>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
