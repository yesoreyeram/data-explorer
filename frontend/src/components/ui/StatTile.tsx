export interface StatTileProps {
  label: string;
  value: string | number;
}

export function StatTile({ label, value }: StatTileProps) {
  return (
    <div className="stat-tile">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
    </div>
  );
}
