import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import { AppStateProvider, useAppState } from './context/AppState';
import { Login } from './pages/Login';
import { Plans } from './pages/Plans';
import { DevInvoice } from './pages/DevInvoice';
import { Checkout } from './pages/Checkout';
import { WebhookSimulator } from './pages/WebhookSimulator';
import './index.css';

const MainNav = () => {
  const { accountId, logout } = useAppState();

  return (
    <nav className="navbar">
      <div className="nav-brand">SaaS Billing</div>
      <div className="nav-links">
        {accountId ? (
          <>
            <Link to="/plans">Plans</Link>
            <Link to="/webhook-simulator">Webhooks</Link>
            <a href="#" onClick={(e) => { e.preventDefault(); logout(); }}>Logout</a>
          </>
        ) : (
          <Link to="/">Login</Link>
        )}
      </div>
    </nav>
  );
};

function App() {
  return (
    <AppStateProvider>
      <Router>
        <div className="layout">
          <MainNav />
          
          <main className="main-content">
            <Routes>
              <Route path="/" element={<Login />} />
              <Route path="/plans" element={<Plans />} />
              <Route path="/invoice" element={<DevInvoice />} />
              <Route path="/checkout" element={<Checkout />} />
              <Route path="/webhook-simulator" element={<WebhookSimulator />} />
            </Routes>
          </main>
        </div>
      </Router>
    </AppStateProvider>
  );
}

export default App;
