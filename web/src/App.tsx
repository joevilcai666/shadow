import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Toast } from '@heroui/react';
import Layout from './components/Layout';
import { LoadingState } from './components/ui';

const MemoryMapPage = lazy(() => import('./pages/MemoryMapPage'));
const Rules = lazy(() => import('./pages/Rules'));
const Memories = lazy(() => import('./pages/Memories'));
const Projects = lazy(() => import('./pages/Projects'));
const Settings = lazy(() => import('./pages/Settings'));
const Welcome = lazy(() => import('./pages/Welcome'));
const RuleDetail = lazy(() => import('./pages/RuleDetail'));
const Review = lazy(() => import('./pages/Review'));
const Conflicts = lazy(() => import('./pages/Conflicts'));

export default function App() {
  return (
    <BrowserRouter>
      <Toast.Provider placement="bottom end" />
      <Suspense fallback={<LoadingState label="Loading Shadow..." />}>
        <Routes>
          {/* Welcome onboarding flow (no sidebar layout) */}
          <Route path="/welcome" element={<Welcome />} />

          {/* Main app with sidebar layout */}
          <Route path="/*" element={
            <Layout>
              <Routes>
                <Route path="/" element={<MemoryMapPage />} />
                <Route path="/rules" element={<Rules />} />
                <Route path="/memories" element={<Memories />} />
                <Route path="/rules/:id" element={<RuleDetail />} />
                <Route path="/review" element={<Review />} />
                <Route path="/conflicts" element={<Conflicts />} />
                <Route path="/projects" element={<Projects />} />
                <Route path="/settings" element={<Settings />} />
              </Routes>
            </Layout>
          } />
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
