import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import MemoryMapPage from './pages/MemoryMapPage';
import Rules from './pages/Rules';
import Projects from './pages/Projects';
import Settings from './pages/Settings';
import Welcome from './pages/Welcome';
import RuleDetail from './pages/RuleDetail';
import Review from './pages/Review';
import Conflicts from './pages/Conflicts';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Welcome onboarding flow (no sidebar layout) */}
        <Route path="/welcome" element={<Welcome />} />

        {/* Main app with sidebar layout */}
        <Route path="/*" element={
          <Layout>
            <Routes>
              <Route path="/" element={<MemoryMapPage />} />
              <Route path="/rules" element={<Rules />} />
              <Route path="/rules/:id" element={<RuleDetail />} />
              <Route path="/review" element={<Review />} />
              <Route path="/conflicts" element={<Conflicts />} />
              <Route path="/projects" element={<Projects />} />
              <Route path="/settings" element={<Settings />} />
            </Routes>
          </Layout>
        } />
      </Routes>
    </BrowserRouter>
  );
}
