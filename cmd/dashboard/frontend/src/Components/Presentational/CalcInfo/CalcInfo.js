import React, { Fragment, useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import { Card } from '@patternfly/react-core/dist/esm/components/Card';
import { CardBody } from '@patternfly/react-core/dist/esm/components/Card/CardBody';
import { CardHeader } from '@patternfly/react-core/dist/esm/components/Card/CardHeader';
import {
    Flex, 
    FlexItem,
    TextContent,
    Text,
    TextVariants,
    PageSection,
    Stack, 
    StackItem,
} from '@patternfly/react-core';
import { getCalculation, reverseSpec } from '../../../Utils/helper';
import ToolbarItems from '../ToolBar/ToolBar'

const CalcInfo = ({ data, selected, handleDeleteCalculation, handleCreateCalculation }) => {
    const [logG, teff] = reverseSpec(selected);
    const calc = getCalculation(data, logG, teff);

    return (<Fragment>
        <ToolbarItems 
            calc={calc} 
            selected={selected}
            handleDeleteCalculation={handleDeleteCalculation} 
            handleCreateCalculation={handleCreateCalculation}
        />
        <PageSection>
            
            <Flex>
                <FlexItem>
                    <Card>
                        <CardHeader>Model</CardHeader>
                        <hr className="divider"/>
                        <CardBody>
                            <TextContent>
                                <Stack hasGutter>
                                    <StackItem><Text component={TextVariants.p}>ATLAS 12</Text></StackItem>
                                    <StackItem> Teff = {teff} K</StackItem>
                                    <StackItem> LoGg = {logG} dex</StackItem>
                                    <StackItem> Metalicity = 0 dex</StackItem>
                                    <StackItem> Vmicro = 0 km/s</StackItem>
                                    <StackItem> status  { calc ? calc.phase : 'N/A'}</StackItem>
                                    { calc && <StackItem> Metadata {calc.metadata.name} </StackItem> }
                                </Stack>

                            </TextContent>
                        </CardBody>
                    </Card>
                </FlexItem>

                <FlexItem>
                    <Card isFlat>
                        <CardHeader>Surface distribution </CardHeader>
                        <hr className="divider"/>
                        <CardBody>Body</CardBody>
                    </Card>
                </FlexItem>
            </Flex>
        </PageSection>
        </Fragment>
    );
};

CalcInfo.propTypes = {
    selected: PropTypes.string.isRequired,
    data: PropTypes.object.isRequired
};

export default CalcInfo;