import React, { Fragment, useEffect, useState} from 'react';
import Head from 'next/head';
import Link from 'next/link';
import CalcInfo from '../components/Presentational/CalcInfo/CalcInfo';

import Calculations from '../services/Calculations'
import Grid from '../components/Presentational/Grid/Grid';
import {
  surfaceGravityAxes,
  effectiveTemperatureAxes,
  TEFF_LOG_GRID
} from '../utils/constants'
import { groupArrayBySpec } from '../utils/helper';

import styles from '../styles/Home.module.css'

export default function Home() {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState([]);
  const [selected, setSelected] = useState();
  const fetchCalculations = () => {
    Calculations.all()
    .then(response => {
        setData(groupArrayBySpec(response.data.items));
    })
    .catch(error => error)
    .finally(() => setLoading(false));
}

const handleDeleteCalculation = (name) => {
    Calculations.delete(name)
    .then(response => {
        fetchCalculations()
    })
}

const handleCreateCalculation = (selected) => {
    Calculations.create(...selected)
    .then(response => {
        fetchCalculations()
    })
}

useEffect(() => {
    fetchCalculations()
}, [])
  return (
    <div className={styles.container}>
      <Head>
        <title>Vega | Dashboard</title>
        <link rel="icon" href="/favicon.ico" />
      </Head>

      <Fragment>
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
        {selected && !loading  && 
            <CalcInfo 
                selected={selected} 
                data={data} 
                handleCreateCalculation={handleCreateCalculation}
                handleDeleteCalculation={handleDeleteCalculation}
            />
        }
    </Fragment>
  
    </div>
  )
}
