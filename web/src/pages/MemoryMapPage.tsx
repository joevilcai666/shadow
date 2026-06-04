// Memory Map 页面入口
// 一期：仅用 mock 数据，未来接入 GET /api/dashboard/map

import { MemoryMap } from '../memory-map/MemoryMap';
import { useNavigate } from 'react-router-dom';

export default function MemoryMapPage() {
  const navigate = useNavigate();
  return (
    <MemoryMap
      onOpenInRules={(id) => navigate(`/rules/${id}`)}
    />
  );
}
