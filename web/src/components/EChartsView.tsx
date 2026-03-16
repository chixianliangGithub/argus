import React from 'react';
import ReactECharts from 'echarts-for-react';

const EChartsView: React.FC<{ option: any; height?: number }> = ({ option, height = 360 }) => {
  if (!option || typeof option !== 'object') return null;
  return <ReactECharts option={option} style={{ height, width: '100%' }} notMerge lazyUpdate />;
};

export default EChartsView;

