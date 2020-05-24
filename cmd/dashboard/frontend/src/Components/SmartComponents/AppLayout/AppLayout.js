import React, { useState } from 'react';
import { Page, PageHeader, PageSectionVariants, PageSection, Brand } from '@patternfly/react-core';
import logo from '../../../App/static/images/vega-logo.png';
import Dashboard from '../Dashboard/Dashboard';

const AppLayout = () => {

  const logoProps = {
    href: '../../assets/vega-logo.png',
    onClick: () => console.log('clicked logo'),
    target: '_blank'
  };

  const Header = (
    <PageHeader
      logo={<Brand src={logo} alt="Vega Logo Black" />}
      logoProps={logoProps}
    />
  );

  return (
    <Page header={Header}>
      <PageSection variant={PageSectionVariants.light}>
        <Dashboard />
      </PageSection>
    </Page>
  );
};

export default AppLayout;
