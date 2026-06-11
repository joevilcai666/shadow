import { useEffect, useState } from 'react';
import { api, type Project } from '../lib/api';
import { FolderOpen } from 'lucide-react';
import { LoadingState, ShadowCard, TagChip } from '../components/ui';

export default function Projects() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listProjects()
      .then(setProjects)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Projects</h1>
      </div>

      {loading ? (
        <LoadingState label="Loading projects..." />
      ) : projects.length === 0 ? (
        <div className="text-center py-12">
          <FolderOpen size={48} className="mx-auto text-gray-700 mb-4" />
          <p className="text-gray-400 mb-2">No projects registered yet</p>
          <p className="text-sm text-gray-500">Run <code className="bg-gray-800 px-1.5 py-0.5 rounded text-xs">shadow start</code> in a project directory to register it.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map(project => (
            <ShadowCard key={project.id} className="p-5 transition-colors hover:border-gray-700">
              <div className="flex items-center gap-3 mb-3">
                <FolderOpen size={20} className="text-purple-400" />
                <h3 className="font-semibold">{project.name}</h3>
              </div>
              <p className="text-xs text-gray-500 font-mono mb-3 truncate">{project.path}</p>
              <div className="flex flex-wrap gap-1">
                {project.agents?.map(agent => (
                  <TagChip key={agent} className="text-purple-300">
                    {agent}
                  </TagChip>
                ))}
              </div>
            </ShadowCard>
          ))}
        </div>
      )}
    </div>
  );
}
