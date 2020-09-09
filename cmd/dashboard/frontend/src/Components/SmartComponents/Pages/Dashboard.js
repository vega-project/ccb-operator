import React, { Fragment, useEffect, useState} from 'react';
import Calculations from '../../../services/Calculations'
import Grid from '../../Presentational/Grid/Grid';
import {
    surfaceGravityAxes,
    effectiveTemperatureAxes,
    TEFF_LOG_GRID
} from '../../../Utils/constants'
import CalcInfo from '../../Presentational/CalcInfo/CalcInfo';
import ToolbarItems from '../../Presentational/ToolBar/ToolBar'
import { groupArrayBySpec } from '../../../Utils/helper';
 
const DashboardPage = () => {
    const [loading, setLoading] = useState(true);
    const [data, setData] = useState(undefined);
    const [selected, setSelected] = useState();

    console.log(selected);
    useEffect(() => {
        Calculations.all()
        .then(response => {
            setData(groupArrayBySpec(response.data.items));
        })
        .catch(error => error)
        .finally(() => setLoading(false));
    }, [])

    return <Fragment>
        { 
            !loading && <Grid
            data={data}
            columns={surfaceGravityAxes}
            rows={effectiveTemperatureAxes}
            yaxis={TEFF_LOG_GRID.yaxis}
            xaxis={TEFF_LOG_GRID.xaxis}
            selected={selected}
            setSelected={setSelected}
        />
        }
        {selected && !loading  && <CalcInfo selected={selected} data={data}/>}
    </Fragment>
}

export default DashboardPage;