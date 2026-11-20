import { BrowserRouter as Router, Routes, Route, Link, Navigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import { AppStateProvider, useAppState } from './context/AppState';
import { Login } from './pages/Login';
import { Plans } from './pages/Plans';
import { DevInvoice } from './pages/DevInvoice';
import { Checkout } from './pages/Checkout';
import { WebhookSimulator } from './pages/WebhookSimulator';

const MainNav = () => {
  const { accessToken, accountId, logout } = useAppState();

  return (
    <nav className="navbar">
      <div className="nav-brand">SaaS Billing</div>
      <div className="nav-links">
        {accessToken && accountId ? (
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

const RequireAccount = ({ children }: { children: ReactNode }) => {
  const { accessToken, accountId } = useAppState();
  return accessToken && accountId ? <>{children}</> : <Navigate to="/" replace />;
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
              <Route path="/plans" element={<RequireAccount><Plans /></RequireAccount>} />
              <Route path="/invoice" element={<RequireAccount><DevInvoice /></RequireAccount>} />
              <Route path="/checkout" element={<RequireAccount><Checkout /></RequireAccount>} />
              <Route path="/webhook-simulator" element={<RequireAccount><WebhookSimulator /></RequireAccount>} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </main>
        </div>
      </Router>
    </AppStateProvider>
  );
}

export default App;
