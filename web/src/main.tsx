import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { appName } from './config/app'
import { applyTheme, readTheme } from './context/theme'

applyTheme(readTheme());
document.title = appName;

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
