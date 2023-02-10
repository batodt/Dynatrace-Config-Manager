import * as React from 'react';
import IconButton from '@mui/material/IconButton';
import { Box, Typography } from '@mui/material';
import { TENANT_KEY_TYPE_TARGET, useTenant, useTenantKey } from '../context/TenantListContext';
import PlayCircleOutlineIcon from '@mui/icons-material/PlayCircleOutline';
import BackupIcon from '@mui/icons-material/Backup';
import { genTenantLabel } from '../credentials/TenantSelector';
import ConfirmAction from '../action/ConfirmAction';
import { useConfirmAction } from './ConfirmHook';

export default function MigrateButton({ label, handlePost, confirm = false, disabled = false }) {

    const { tenantKey: tenantKeyTarget } = useTenantKey(TENANT_KEY_TYPE_TARGET)
    const { tenant: tenantTarget } = useTenant(tenantKeyTarget)
    const { open, handleClickOpen, handleClose } = useConfirmAction()

    const tenantLabel = React.useMemo(() => {
        return genTenantLabel({ ...tenantTarget, 'key': tenantKeyTarget }, "Target")
    }, [tenantTarget, tenantKeyTarget])

    const handleClickAction = React.useMemo(() => {
        if (confirm === true) {
            return handleClickOpen
        } else {
            return handlePost
        }
    }, [confirm, handlePost, handleClickOpen])

    const button = React.useMemo(() => {

        const props = { fontSize: 'large' }
        let buttonIcon = null
        let color = null

        if (confirm === true) {
            buttonIcon = (<BackupIcon {...props} />)
            color = 'success'
        } else {
            buttonIcon = (<PlayCircleOutlineIcon {...props} />)
            color = 'primary'
        }

        return (
            <IconButton onClick={handleClickAction} color={color} disabled={disabled}>
                {buttonIcon}
                <Typography sx={{ ml: 1 }}>{label}</Typography>
            </IconButton>
        )
    }, [confirm, label, handleClickAction, disabled])

    const confirmDialog = React.useMemo(() => {

        if (confirm === true) {
            const descLabel = "This will send API Requests, updating your tenant's configuration."
            return (
                <ConfirmAction open={open} handleClose={handleClose} label={label}
                    descLabel={descLabel} tenantLabel={tenantLabel} handlePost={handlePost} />
            )
        }

        return null


    }, [confirm, handleClose, handlePost, label, open, tenantLabel])

    return (
        <Box sx={{ my: 1 }}>
            {button}
            {confirmDialog}
        </Box>
    );
}
