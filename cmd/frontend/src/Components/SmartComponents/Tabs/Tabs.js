import React, { useState } from 'react';
import { Tabs, Tab, TabsVariant, TabContent } from '@patternfly/react-core';
import Table from '../../PresentationalComponents/Table/Table';
import { effectiveTemperatureAxes, surfaceGravityAxes, microturbulanceAxes } from '../../../constants';

const DashboardTabs = () => {
  const [activeTabKey, setActiveTabKey] = useState(0);

  return (
    <Tabs isFilled activeKey={activeTabKey} onSelect={(evt, index) => setActiveTabKey(index)}>
      <Tab eventKey={0} title="Teff/Log g">
        <Table columns={surfaceGravityAxes} yaxis="log g [cgs]" xaxis="Teff" rows={effectiveTemperatureAxes} />
      </Tab>
      <Tab eventKey={1} title="Teff/Vturb">
        <Table columns={microturbulanceAxes} yaxis="log g [cgs]" xaxis="Vturb" rows={effectiveTemperatureAxes} />
      </Tab>
      <Tab eventKey={2} title="Ω/i">
        Tab 3 section
      </Tab>
    </Tabs>
  );
};

export default DashboardTabs;
