import { render } from 'solid-js/web'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import App from './app'
import { initTheme } from './theme'
import './styles/index.css'

initTheme()

render(() => <App />, document.getElementById('app')!)