import React, { useState } from 'react';
import { Tabs, Tab } from '@patternfly/react-core';
import Table from '../../PresentationalComponents/Table/Table';
import {
    TEFF_LOG_GRID,
    TEFF_TURB_GRID,
    effectiveTemperatureAxes,
    surfaceGravityAxes,
    microturbulanceAxes
} from '../../../constants';

const DashboardTabs = () => {
    const [activeTabKey, setActiveTabKey] = useState(0);

    return (
        <Tabs isFilled activeKey={activeTabKey} onSelect={(evt, index) => setActiveTabKey(index)}>
            <Tab eventKey={0} title={TEFF_LOG_GRID.title}>
                <Table
                    columns={surfaceGravityAxes}
                    rows={effectiveTemperatureAxes}
                    yaxis={TEFF_LOG_GRID.yaxis}
                    xaxis={TEFF_LOG_GRID.xaxis}
                />
            </Tab>

            <Tab eventKey={1} title={TEFF_TURB_GRID.title}>
                <Table
                    columns={microturbulanceAxes}
                    rows={effectiveTemperatureAxes}
                    yaxis={TEFF_TURB_GRID.yaxis}
                    xaxis={TEFF_TURB_GRID.xaxis}
                />
            </Tab>
        </Tabs>
    );
};

export default DashboardTabs;
