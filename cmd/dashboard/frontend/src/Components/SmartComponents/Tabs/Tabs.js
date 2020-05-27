import React, { useState, useEffect } from 'react';
import { Tabs, Tab } from '@patternfly/react-core';
import Table from '../../PresentationalComponents/Table/Table';
import EmptyStateSpinner from '../../PresentationalComponents/EmptyStateSpinner/EmptyStateSpinner';
import Calculations from '../../../../services/Calculations';
import {
    TEFF_LOG_GRID,
    TEFF_TURB_GRID,
    effectiveTemperatureAxes,
    surfaceGravityAxes,
    microturbulanceAxes
} from '../../../constants';

const DashboardTabs = () => {
    const [activeTabKey, setActiveTabKey] = useState(0);
    const [data, setData] = useState(undefined);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        Calculations.all()
        .then(response => {
            setData(response.data);
        })
        .catch(error => error)
        .finally(() => setLoading(false));

    }, []);

    return (
        <Tabs isFilled activeKey={activeTabKey} onSelect={(evt, index) => setActiveTabKey(index)}>
            <Tab eventKey={0} title={TEFF_LOG_GRID.title}>
                {loading ? <EmptyStateSpinner/>
                    : <Table
                        data={data}
                        columns={surfaceGravityAxes}
                        rows={effectiveTemperatureAxes}
                        yaxis={TEFF_LOG_GRID.yaxis}
                        xaxis={TEFF_LOG_GRID.xaxis}
                    />
                }

            </Tab>
        </Tabs>
    );
};

export default DashboardTabs;
