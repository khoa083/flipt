import { useState } from 'react';

export default function ChipList({
  values,
  maxItemCount = 5,
  showAll = false
}: {
  values: (string | number)[];
  maxItemCount?: number;
  showAll?: boolean;
}) {
  const [showAllValues, setShowAllValues] = useState<boolean>(showAll);

  const visibleValues = showAllValues ? values : values.slice(0, maxItemCount);
  return (
    <div className="flex flex-wrap gap-2">
      {visibleValues.map((value, i) => (
        <div
          className="max-w-32 truncate rounded-sm bg-gray-200 px-1.5 py-0.5 text-gray-900"
          key={i}
        >
          {value}
        </div>
      ))}
      {!showAll && values.length > maxItemCount && (
        <button
          className="rounded-sm px-1.5 text-gray-700"
          onClick={() => setShowAllValues((b) => !b)}
        >
          {showAllValues ? 'show less' : '...'}
        </button>
      )}
    </div>
  );
}
